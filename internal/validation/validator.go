package validation

import (
	"fmt"
	"sort"
	"strings"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

type expressionParser interface {
	Parse(expr string) error
	Identifiers(expr string) ([]string, error)
}

type Validator struct {
	catalog   catalog.Catalog
	evaluator expressionParser
}

func New(cat catalog.Catalog, evaluator expressionParser) *Validator {
	return &Validator{catalog: cat, evaluator: evaluator}
}

func (v *Validator) Validate(workflow kernel.WorkflowResource) []Issue {
	var issues []Issue

	trigger, ok := v.catalog.GetTriggerManifest(workflow.Spec.Trigger.Type)
	if !ok {
		issues = append(issues, Issue{
			Code:    "unknown_trigger",
			Level:   IssueLevelError,
			Message: fmt.Sprintf("Trigger %q is not registered.", workflow.Spec.Trigger.Type),
			Path:    "spec.trigger.type",
			Hint:    "Register the trigger plugin or change the workflow trigger type.",
		})
	}

	vars := map[string]catalog.Schema{
		"inputs": {Type: "object"},
	}
	if workflow.Spec.Trigger.As != "" {
		vars[workflow.Spec.Trigger.As] = trigger.EventSchema
	}

	seenAliases := make(map[string]struct{})
	issues = append(issues, v.validateSteps(workflow.Spec.Steps, "spec.steps", vars, seenAliases)...)
	return issues
}

func (v *Validator) validateSteps(steps []kernel.StepDef, path string, vars map[string]catalog.Schema, seenAliases map[string]struct{}) []Issue {
	var issues []Issue
	for i, step := range steps {
		stepPath := fmt.Sprintf("%s[%d]", path, i)
		if step.If != "" {
			if err := v.evaluator.Parse(step.If); err != nil {
				issues = append(issues, Issue{
					Code:    "invalid_expression",
					Level:   IssueLevelError,
					Message: fmt.Sprintf("Condition %q is invalid: %v", step.If, err),
					Path:    stepPath + ".if",
					Hint:    "Check field names and supported operators.",
				})
			} else {
				issues = append(issues, v.unknownIdentifierIssues(step.If, stepPath+".if", vars)...)
			}
		}

		if step.Do != "" {
			manifest, ok := v.catalog.GetOperationManifest(step.Do)
			if !ok {
				issues = append(issues, Issue{
					Code:    "unknown_operation",
					Level:   IssueLevelError,
					Message: fmt.Sprintf("Operation %q is not registered.", step.Do),
					Path:    stepPath + ".do",
					Hint:    "Register the plugin or change the operation type.",
				})
			} else {
				issues = append(issues, validateRequiredInputs(step.With, manifest, stepPath+".with")...)
				issues = append(issues, v.referenceIssues(step.With, stepPath+".with", vars)...)
				if manifest.SideEffect {
					issues = append(issues, Issue{
						Code:    "side_effect_operation",
						Level:   IssueLevelWarning,
						Message: fmt.Sprintf("Operation %q has side effects.", step.Do),
						Path:    stepPath + ".do",
						Hint:    "Prefer dry-run before enabling live execution.",
					})
				}
				if step.As != "" {
					if _, exists := seenAliases[step.As]; exists {
						issues = append(issues, Issue{
							Code:    "alias_collision",
							Level:   IssueLevelError,
							Message: fmt.Sprintf("Alias %q is already defined.", step.As),
							Path:    stepPath + ".as",
							Hint:    "Use a unique alias for each operation output.",
						})
					}
					seenAliases[step.As] = struct{}{}
					vars[step.As] = manifest.OutputSchema
				}
			}
		}

		if len(step.Then) > 0 {
			issues = append(issues, v.validateSteps(step.Then, stepPath+".then", copyVars(vars), copyAliases(seenAliases))...)
		}
		if len(step.Else) > 0 {
			issues = append(issues, v.validateSteps(step.Else, stepPath+".else", copyVars(vars), copyAliases(seenAliases))...)
		}
	}
	return issues
}

func validateRequiredInputs(with map[string]any, manifest catalog.OperationManifest, path string) []Issue {
	var issues []Issue
	for _, required := range manifest.InputSchema.Required {
		if _, ok := with[required]; ok {
			continue
		}
		issues = append(issues, Issue{
			Code:    "missing_input",
			Level:   IssueLevelError,
			Message: fmt.Sprintf("Missing required input %q for operation %q.", required, manifest.Type),
			Path:    path,
			Hint:    "Provide all required operation inputs in the step's with block.",
		})
	}
	return issues
}

func (v *Validator) unknownIdentifierIssues(expr, path string, vars map[string]catalog.Schema) []Issue {
	identifiers, err := v.evaluator.Identifiers(expr)
	if err != nil {
		return nil
	}
	return unknownPathIssues(identifiers, path, vars)
}

func (v *Validator) referenceIssues(value any, path string, vars map[string]catalog.Schema) []Issue {
	var issues []Issue
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			issues = append(issues, v.referenceIssues(typed[key], path+"."+key, vars)...)
		}
	case []any:
		for i, item := range typed {
			issues = append(issues, v.referenceIssues(item, fmt.Sprintf("%s[%d]", path, i), vars)...)
		}
	case string:
		if strings.HasPrefix(typed, "$") {
			issues = append(issues, unknownPathIssues([]string{strings.TrimPrefix(typed, "$")}, path, vars)...)
		}
	}
	return issues
}

func unknownPathIssues(paths []string, path string, vars map[string]catalog.Schema) []Issue {
	var issues []Issue
	for _, ref := range paths {
		if isKnownPath(ref, vars) {
			continue
		}
		issues = append(issues, Issue{
			Code:    "unknown_variable",
			Level:   IssueLevelError,
			Message: fmt.Sprintf("Reference %q is not defined in the current workflow scope.", ref),
			Path:    path,
			Hint:    "Check trigger fields, input names, and previous step aliases.",
		})
	}
	return issues
}

func isKnownPath(path string, vars map[string]catalog.Schema) bool {
	parts := strings.Split(path, ".")
	if len(parts) > 1 && parts[0] == "inputs" {
		return true
	}
	rootSchema, ok := vars[parts[0]]
	if !ok {
		return false
	}
	current := rootSchema
	for _, part := range parts[1:] {
		next, ok := current.Properties[part]
		if !ok {
			return false
		}
		current = next
	}
	return true
}

func copyVars(input map[string]catalog.Schema) map[string]catalog.Schema {
	out := make(map[string]catalog.Schema, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func copyAliases(input map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}
