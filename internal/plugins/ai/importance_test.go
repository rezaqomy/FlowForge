package ai

import (
	"context"
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func TestRegisterManifestMatchesHandler(t *testing.T) {
	reg := kernel.NewRegistry()
	cat := catalog.New()
	Register(reg, cat)

	op, ok := reg.GetOperation("ai.importance")
	if !ok {
		t.Fatalf("expected ai.importance operation")
	}
	manifest, ok := cat.GetOperationManifest("ai.importance")
	if !ok {
		t.Fatalf("expected ai.importance manifest")
	}
	result, err := op.Run(context.Background(), map[string]any{
		"text":    "urgent payment issue",
		"context": "customer support",
	}, kernel.OperationMeta{Mode: kernel.RunModeDryRun})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if manifest.SideEffect {
		t.Fatalf("manifest side effect = true, want false")
	}
	if _, ok := result.Output["important"].(bool); !ok {
		t.Fatalf("expected important bool output")
	}
}
