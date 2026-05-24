package runner

import (
	"context"

	"flowforge/internal/kernel"
)

type Runner struct {
	engine *kernel.Engine
}

func New(engine *kernel.Engine) *Runner {
	return &Runner{engine: engine}
}

func (r *Runner) Run(ctx context.Context, req kernel.RunRequest) (*Result, error) {
	sink := &MemoryEventSink{}
	req.Sink = kernel.NewFanoutSink(req.Sink, sink)

	runResult, err := r.engine.Run(ctx, req)
	if err != nil {
		return &Result{Run: runResult, Events: sink.Events}, err
	}
	return &Result{Run: runResult, Events: sink.Events}, nil
}
