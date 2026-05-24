package kernel

import "testing"

func TestExpressionEvaluatorEvalBool(t *testing.T) {
	scope := NewScope()
	scope.Set("message", map[string]any{
		"sender_id": "u_123",
		"text":      "urgent payment issue",
	})
	scope.Set("inputs", map[string]any{
		"target_contacts": []any{"u_123", "u_456"},
	})
	scope.Set("analysis", map[string]any{
		"important":  true,
		"confidence": 0.91,
	})

	tests := []struct {
		name string
		expr string
		want bool
	}{
		{name: "in operator", expr: "message.sender_id in inputs.target_contacts", want: true},
		{name: "boolean field", expr: "analysis.important", want: true},
		{name: "comparison", expr: "analysis.confidence > 0.8", want: true},
		{name: "contains", expr: "message.text contains \"urgent\"", want: true},
		{name: "and", expr: "analysis.important and analysis.confidence >= 0.9", want: true},
		{name: "not", expr: "not (analysis.confidence < 0.9)", want: true},
	}

	evaluator := NewEvaluator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluator.EvalBool(tt.expr, scope)
			if err != nil {
				t.Fatalf("EvalBool() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("EvalBool() = %v, want %v", got, tt.want)
			}
		})
	}
}
