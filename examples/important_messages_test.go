package examples

import (
	"context"
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
	"flowforge/internal/plugins/ai"
	"flowforge/internal/plugins/storage"
	"flowforge/internal/plugins/telegram"
)

func TestImportantMessagesWorkflowLoadsAndDryRuns(t *testing.T) {
	data, err := os.ReadFile("important_messages.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var workflow kernel.WorkflowResource
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	reg := kernel.NewRegistry()
	cat := catalog.New()
	ai.Register(reg, cat)
	telegram.Register(reg, cat)
	storage.Register(reg, cat)

	result, err := kernel.NewEngine(reg, kernel.NewEvaluator(), kernel.NewResolver()).Run(context.Background(), kernel.RunRequest{
		Workflow: workflow,
		Event: kernel.Event{
			Type: "telegram.message",
			Payload: map[string]any{
				"id":        "msg_1",
				"sender_id": "u_123",
				"chat_id":   "chat_1",
				"text":      "urgent payment issue",
			},
		},
		Inputs: map[string]any{
			"target_contacts":  []any{"u_123"},
			"business_context": "customer support",
			"admin":            "admin_chat",
		},
		Mode: kernel.RunModeDryRun,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != kernel.RunStatusCompleted {
		t.Fatalf("status = %s, want %s", result.Status, kernel.RunStatusCompleted)
	}
}
