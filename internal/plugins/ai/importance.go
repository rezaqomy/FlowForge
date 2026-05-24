package ai

import (
	"context"
	"fmt"
	"strings"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func Register(reg *kernel.Registry, cat *catalog.Registry) {
	reg.Register("ai.importance", ImportanceOperation{})
	cat.RegisterManifest(catalog.PluginManifest{
		Name: "ai",
		Operations: []catalog.OperationManifest{
			{
				Type:        "ai.importance",
				Description: "Classifies whether a message is important.",
				InputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"text":    {Type: "string"},
						"context": {Type: "string"},
					},
					Required: []string{"text", "context"},
				},
				OutputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"important":  {Type: "boolean"},
						"reason":     {Type: "string"},
						"confidence": {Type: "number"},
					},
					Required: []string{"important", "reason", "confidence"},
				},
				SideEffect: false,
				Display:    catalog.DisplayHints{Label: "ai.importance"},
			},
		},
	})
}

type ImportanceOperation struct{}

func (ImportanceOperation) Run(_ context.Context, input map[string]any, _ kernel.OperationMeta) (kernel.OperationResult, error) {
	text, ok := input["text"].(string)
	if !ok {
		return kernel.OperationResult{}, fmt.Errorf("text must be a string")
	}
	contextValue, ok := input["context"].(string)
	if !ok {
		return kernel.OperationResult{}, fmt.Errorf("context must be a string")
	}

	normalized := strings.ToLower(text + " " + contextValue)
	important := strings.Contains(normalized, "urgent") || strings.Contains(normalized, "payment")
	reason := "No important signal detected"
	confidence := 0.24
	if important {
		reason = "Detected important support signal"
		confidence = 0.91
	}

	return kernel.OperationResult{
		Output: map[string]any{
			"important":  important,
			"reason":     reason,
			"confidence": confidence,
		},
	}, nil
}
