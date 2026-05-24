package telegram

import (
	"context"
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func TestDryRunDoesNotTriggerRealSideEffect(t *testing.T) {
	reg := kernel.NewRegistry()
	cat := catalog.New()
	Register(reg, cat)

	op, ok := reg.GetOperation("telegram.send")
	if !ok {
		t.Fatalf("expected telegram.send operation")
	}
	send, ok := op.(*SendOperation)
	if !ok {
		t.Fatalf("unexpected operation type %T", op)
	}
	manifest, ok := cat.GetOperationManifest("telegram.send")
	if !ok {
		t.Fatalf("expected telegram.send manifest")
	}
	if !manifest.SideEffect {
		t.Fatalf("telegram.send should be marked as side-effecting")
	}

	result, err := send.Run(context.Background(), map[string]any{
		"to":   "admin",
		"text": "hello",
	}, kernel.OperationMeta{Mode: kernel.RunModeDryRun})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(send.Sent) != 0 {
		t.Fatalf("dry-run should not record a live send")
	}
	if _, ok := result.Output["message_id"].(string); !ok {
		t.Fatalf("expected message_id output")
	}
}
