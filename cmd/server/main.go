package main

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
	"flowforge/internal/plugins/ai"
	"flowforge/internal/plugins/storage"
	"flowforge/internal/plugins/telegram"
	"flowforge/internal/render"
	"flowforge/internal/runner"
)

func main() {
	reg := kernel.NewRegistry()
	cat := catalog.New()
	ai.Register(reg, cat)
	telegram.Register(reg, cat)
	storage.Register(reg, cat)

	workflowBytes, err := os.ReadFile("examples/important_messages.yaml")
	if err != nil {
		panic(err)
	}
	var workflow kernel.WorkflowResource
	if err := yaml.Unmarshal(workflowBytes, &workflow); err != nil {
		panic(err)
	}

	fmt.Println(render.NewPseudoCodeRenderer().Render(workflow, cat))

	engine := kernel.NewEngine(reg, kernel.NewEvaluator(), kernel.NewResolver())
	result, err := runner.New(engine).Run(context.Background(), kernel.RunRequest{
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
		panic(err)
	}
	fmt.Printf("\nrun status: %s (%d events)\n", result.Run.Status, len(result.Events))
}
