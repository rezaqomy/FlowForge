package kernel

import (
	"fmt"
	"strings"
)

type Resolver interface {
	Resolve(value any, scope *Scope) (any, error)
}

type PathResolver struct{}

func NewResolver() *PathResolver {
	return &PathResolver{}
}

func (r *PathResolver) Resolve(value any, scope *Scope) (any, error) {
	switch typed := value.(type) {
	case string:
		if !strings.HasPrefix(typed, "$") {
			return typed, nil
		}
		resolved, ok := scope.GetPath(strings.TrimPrefix(typed, "$"))
		if !ok {
			return nil, fmt.Errorf("resolve path %q: %w", typed, ErrPathNotFound)
		}
		return resolved, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			resolved, err := r.Resolve(v, scope)
			if err != nil {
				return nil, err
			}
			out[k] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(typed))
		for i, v := range typed {
			resolved, err := r.Resolve(v, scope)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return value, nil
	}
}
