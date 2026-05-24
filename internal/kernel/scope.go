package kernel

import "strings"

type Scope struct {
	values map[string]any
}

func NewScope() *Scope {
	return &Scope{values: make(map[string]any)}
}

func (s *Scope) GetPath(path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	current, ok := s.values[parts[0]]
	if !ok {
		return nil, false
	}
	for _, part := range parts[1:] {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func (s *Scope) Set(name string, value any) {
	s.values[name] = value
}

func (s *Scope) Snapshot() map[string]any {
	return cloneMap(s.values)
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneSlice(input []any) []any {
	out := make([]any, len(input))
	for i, v := range input {
		out[i] = cloneValue(v)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		return cloneSlice(typed)
	default:
		return typed
	}
}
