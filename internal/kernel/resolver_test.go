package kernel

import "testing"

func TestPathResolverResolve(t *testing.T) {
	scope := NewScope()
	scope.Set("message", map[string]any{"text": "urgent payment issue"})
	scope.Set("inputs", map[string]any{"admin": "admin_chat"})

	tests := []struct {
		name  string
		input any
		want  any
	}{
		{name: "message path", input: "$message.text", want: "urgent payment issue"},
		{name: "inputs path", input: "$inputs.admin", want: "admin_chat"},
		{name: "recursive map", input: map[string]any{"to": "$inputs.admin"}, want: map[string]any{"to": "admin_chat"}},
		{name: "literal string", input: "hello", want: "hello"},
	}

	resolver := NewResolver()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolver.Resolve(tt.input, scope)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if gotMap, ok := got.(map[string]any); ok {
				wantMap := tt.want.(map[string]any)
				if gotMap["to"] != wantMap["to"] {
					t.Fatalf("Resolve() map = %#v, want %#v", gotMap, wantMap)
				}
				return
			}
			if got != tt.want {
				t.Fatalf("Resolve() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
