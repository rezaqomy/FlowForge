package telegram

import (
	"context"
	"fmt"

	"flowforge/internal/kernel"
)

type SendOperation struct {
	Sent []map[string]any
}

func (s *SendOperation) Run(_ context.Context, input map[string]any, meta kernel.OperationMeta) (kernel.OperationResult, error) {
	to, ok := input["to"].(string)
	if !ok {
		return kernel.OperationResult{}, fmt.Errorf("to must be a string")
	}
	text, ok := input["text"].(string)
	if !ok {
		return kernel.OperationResult{}, fmt.Errorf("text must be a string")
	}

	if meta.Mode == kernel.RunModeLive {
		s.Sent = append(s.Sent, map[string]any{"to": to, "text": text})
	}
	return kernel.OperationResult{
		Output: map[string]any{
			"message_id": "dryrun-msg-1",
		},
	}, nil
}
