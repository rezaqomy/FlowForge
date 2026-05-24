package app

import (
	"context"

	"flowforge/internal/kernel"
	"flowforge/internal/runner"
)

type RunService struct {
	runner *runner.Runner
}

func NewRunService(r *runner.Runner) *RunService {
	return &RunService{runner: r}
}

func (s *RunService) Run(ctx context.Context, req kernel.RunRequest) (*runner.Result, error) {
	return s.runner.Run(ctx, req)
}
