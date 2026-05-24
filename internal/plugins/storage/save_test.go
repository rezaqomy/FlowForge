package storage

import (
	"context"
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func TestInputOutputShapeIsStable(t *testing.T) {
	reg := kernel.NewRegistry()
	cat := catalog.New()
	Register(reg, cat)

	op, ok := reg.GetOperation("storage.save")
	if !ok {
		t.Fatalf("expected storage.save operation")
	}
	save, ok := op.(*SaveOperation)
	if !ok {
		t.Fatalf("unexpected operation type %T", op)
	}
	result, err := save.Run(context.Background(), map[string]any{
		"key":   "message",
		"value": "payload",
	}, kernel.OperationMeta{Mode: kernel.RunModeLive})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Output["saved"] != true {
		t.Fatalf("saved output = %#v", result.Output)
	}
	if save.data["message"] != "payload" {
		t.Fatalf("live mode should persist data")
	}
}
