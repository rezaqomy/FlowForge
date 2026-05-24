package runner

import (
	"context"
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
	"flowforge/internal/plugins/ai"
	"flowforge/internal/plugins/telegram"
)

func TestRunnerCapturesTraceEvents(t *testing.T) {
	reg := kernel.NewRegistry()
	cat := catalog.New()
	ai.Register(reg, cat)
	telegram.Register(reg, cat)

	engine := kernel.NewEngine(reg, kernel.NewEvaluator(), kernel.NewResolver())
	result, err := New(engine).Run(context.Background(), kernel.RunRequest{
		Workflow: kernel.WorkflowResource{
			Metadata: kernel.Metadata{Name: "wf"},
			Spec: kernel.WorkflowSpec{
				Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
				Steps: []kernel.StepDef{
					{
						ID: "analyze",
						Do: "ai.importance",
						As: "analysis",
						With: map[string]any{
							"text":    "$message.text",
							"context": "$inputs.business_context",
						},
					},
				},
			},
		},
		Event: kernel.Event{
			Type:    "telegram.message",
			Payload: map[string]any{"text": "urgent"},
		},
		Inputs: map[string]any{"business_context": "support"},
		Mode:   kernel.RunModeDryRun,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Events) == 0 {
		t.Fatalf("expected runner events")
	}
}
