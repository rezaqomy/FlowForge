package storage

import (
	"context"
	"fmt"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func Register(reg *kernel.Registry, cat *catalog.Registry) {
	reg.Register("storage.save", &SaveOperation{data: make(map[string]any)})
	cat.RegisterManifest(catalog.PluginManifest{
		Name: "storage",
		Operations: []catalog.OperationManifest{
			{
				Type:        "storage.save",
				Description: "Saves a value in in-memory storage.",
				InputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"key":   {Type: "string"},
						"value": {Type: "string"},
					},
					Required: []string{"key", "value"},
				},
				OutputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"saved": {Type: "boolean"},
					},
					Required: []string{"saved"},
				},
				SideEffect: true,
				Display:    catalog.DisplayHints{Label: "storage.save"},
			},
		},
	})
}

type SaveOperation struct {
	data map[string]any
}

func (s *SaveOperation) Run(_ context.Context, input map[string]any, meta kernel.OperationMeta) (kernel.OperationResult, error) {
	key, ok := input["key"].(string)
	if !ok {
		return kernel.OperationResult{}, fmt.Errorf("key must be a string")
	}
	if meta.Mode == kernel.RunModeLive {
		s.data[key] = input["value"]
	}
	return kernel.OperationResult{Output: map[string]any{"saved": true}}, nil
}
