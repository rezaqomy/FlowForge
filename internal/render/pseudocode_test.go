package render

import (
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func TestRenderWorkflowToPseudoCode(t *testing.T) {
	renderer := NewPseudoCodeRenderer()
	cat := catalog.New()
	cat.RegisterManifest(catalog.PluginManifest{
		Name: "example",
		Operations: []catalog.OperationManifest{
			{Type: "ai.importance", Display: catalog.DisplayHints{Label: "ai.importance"}},
			{Type: "telegram.send", Display: catalog.DisplayHints{Label: "telegram.send"}},
		},
	})

	workflow := kernel.WorkflowResource{
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
			Steps: []kernel.StepDef{
				{
					ID: "check",
					If: "message.sender_id in inputs.target_contacts",
					Then: []kernel.StepDef{
						{
							ID: "analyze",
							Do: "ai.importance",
							As: "analysis",
							With: map[string]any{
								"text":    "$message.text",
								"context": "$inputs.business_context",
							},
						},
						{
							ID: "send",
							If: "analysis.important",
							Do: "telegram.send",
							With: map[string]any{
								"to":   "$inputs.admin",
								"text": "$message.text",
							},
						},
					},
				},
			},
		},
	}

	got := renderer.Render(workflow, cat)
	want := "on telegram.message as message:\n\n    if message.sender_id in target_contacts:\n        analysis = ai.importance(\n            context = business_context,\n            text = message.text,\n        )\n        if analysis.important:\n            telegram.send(\n                text = message.text,\n                to = admin,\n            )"
	if got != want {
		t.Fatalf("Render() =\n%s\nwant\n%s", got, want)
	}
}
