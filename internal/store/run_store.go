package store

import "flowforge/internal/runner"

type RunStore interface {
	Save(result *runner.Result) error
}
