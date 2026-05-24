package runner

import "flowforge/internal/kernel"

type Result struct {
	Run    *kernel.RunResult
	Events []kernel.ExecutionEvent
}
