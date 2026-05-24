package validation

import (
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
	"flowforge/internal/plugins/ai"
	"flowforge/internal/plugins/storage"
	"flowforge/internal/plugins/telegram"
)

func testValidator() *Validator {
	cat := catalog.New()
	reg := kernel.NewRegistry()
	ai.Register(reg, cat)
	telegram.Register(reg, cat)
	storage.Register(reg, cat)
	return New(cat, kernel.NewEvaluator())
}

func TestUnknownOperationProducesIssue(t *testing.T) {
	issues := testValidator().Validate(kernel.WorkflowResource{
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
			Steps:   []kernel.StepDef{{ID: "bad", Do: "missing.op"}},
		},
	})
	assertHasCode(t, issues, "unknown_operation")
}

func TestMissingInputProducesIssue(t *testing.T) {
	issues := testValidator().Validate(kernel.WorkflowResource{
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
			Steps:   []kernel.StepDef{{ID: "send", Do: "telegram.send", With: map[string]any{"to": "$inputs.admin"}}},
		},
	})
	assertHasCode(t, issues, "missing_input")
}

func TestUnknownVariableProducesIssue(t *testing.T) {
	issues := testValidator().Validate(kernel.WorkflowResource{
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
			Steps:   []kernel.StepDef{{ID: "check", If: "message.unknown == true"}},
		},
	})
	assertHasCode(t, issues, "unknown_variable")
}

func TestInvalidExpressionProducesIssue(t *testing.T) {
	issues := testValidator().Validate(kernel.WorkflowResource{
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
			Steps:   []kernel.StepDef{{ID: "check", If: "message.sender_id and"}},
		},
	})
	assertHasCode(t, issues, "invalid_expression")
}

func assertHasCode(t *testing.T, issues []Issue, code string) {
	t.Helper()
	for _, issue := range issues {
		if issue.Code == code {
			return
		}
	}
	t.Fatalf("expected issue code %q, got %#v", code, issues)
}
