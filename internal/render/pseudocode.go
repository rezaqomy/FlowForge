package render

import (
	"fmt"
	"sort"
	"strings"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

type PseudoCodeRenderer struct{}

func NewPseudoCodeRenderer() *PseudoCodeRenderer {
	return &PseudoCodeRenderer{}
}

func (r *PseudoCodeRenderer) Render(workflow kernel.WorkflowResource, cat catalog.Catalog) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("on %s as %s:", workflow.Spec.Trigger.Type, workflow.Spec.Trigger.As))
	lines = append(lines, "")
	lines = append(lines, r.renderSteps(workflow.Spec.Steps, cat, 1)...)
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

func (r *PseudoCodeRenderer) renderSteps(steps []kernel.StepDef, cat catalog.Catalog, depth int) []string {
	var lines []string
	indent := strings.Repeat("    ", depth)

	for _, step := range steps {
		if step.If != "" {
			lines = append(lines, fmt.Sprintf("%sif %s:", indent, humanizeRef(step.If)))
			if step.Do != "" {
				lines = append(lines, r.renderOperation(step, cat, depth+1)...)
			}
			if len(step.Then) > 0 {
				lines = append(lines, r.renderSteps(step.Then, cat, depth+1)...)
			}
			if len(step.Else) > 0 {
				lines = append(lines, fmt.Sprintf("%selse:", indent))
				lines = append(lines, r.renderSteps(step.Else, cat, depth+1)...)
			}
			continue
		}

		if step.Do != "" {
			lines = append(lines, r.renderOperation(step, cat, depth)...)
		}

		if len(step.Then) > 0 {
			lines = append(lines, r.renderSteps(step.Then, cat, depth)...)
		}
	}
	return trimTrailingBlank(lines)
}

func (r *PseudoCodeRenderer) renderOperation(step kernel.StepDef, cat catalog.Catalog, depth int) []string {
	indent := strings.Repeat("    ", depth)
	manifest, ok := cat.GetOperationManifest(step.Do)
	label := step.Do
	if ok && manifest.Display.Label != "" {
		label = manifest.Display.Label
	}

	keys := make([]string, 0, len(step.With))
	for key := range step.With {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	head := fmt.Sprintf("%s%s(", indent, label)
	if step.As != "" {
		head = fmt.Sprintf("%s%s = %s(", indent, step.As, label)
	}
	lines := []string{head}
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s    %s = %s,", indent, key, humanizeValue(step.With[key])))
	}
	lines = append(lines, fmt.Sprintf("%s)", indent))
	return lines
}

func humanizeValue(value any) string {
	switch typed := value.(type) {
	case string:
		if strings.HasPrefix(typed, "$") {
			return humanizeRef(strings.TrimPrefix(typed, "$"))
		}
		return fmt.Sprintf("%q", typed)
	default:
		return fmt.Sprint(value)
	}
}

func humanizeRef(value string) string {
	return strings.ReplaceAll(value, "inputs.", "")
}

func trimTrailingBlank(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
