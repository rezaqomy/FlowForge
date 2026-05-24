package kernel

import (
	"context"
	"fmt"
	"strconv"
	"time"
)

type Engine struct {
	Ops       OperationRegistry
	Evaluator Evaluator
	Resolver  Resolver
	now       func() time.Time
	newRunID  func() string
}

func NewEngine(ops OperationRegistry, evaluator Evaluator, resolver Resolver) *Engine {
	return &Engine{
		Ops:       ops,
		Evaluator: evaluator,
		Resolver:  resolver,
		now:       time.Now().UTC,
		newRunID: func() string {
			return "run_" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
		},
	}
}

func (e *Engine) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	if req.Workflow.Spec.Trigger.As == "" {
		return nil, fmt.Errorf("%w: trigger alias is required", ErrInvalidWorkflow)
	}

	runID := e.newRunID()
	collector := &CollectingSink{}
	sink := NewFanoutSink(collector, req.Sink)
	scope := NewScope()
	scope.Set(req.Workflow.Spec.Trigger.As, cloneMap(req.Event.Payload))
	scope.Set("inputs", cloneMap(req.Inputs))

	e.emit(sink, ExecutionEvent{
		Type:      "run.started",
		RunID:     runID,
		Message:   "run started",
		Timestamp: e.now(),
		Data: map[string]any{
			"workflow": req.Workflow.Metadata.Name,
			"mode":     req.Mode,
		},
	})

	if err := e.runSteps(ctx, req, runID, sink, scope, req.Workflow.Spec.Steps); err != nil {
		e.emit(sink, ExecutionEvent{
			Type:      "run.failed",
			RunID:     runID,
			Message:   err.Error(),
			Timestamp: e.now(),
		})
		return &RunResult{
			Status: RunStatusFailed,
			Vars:   scope.Snapshot(),
			Events: collector.Events,
		}, err
	}

	e.emit(sink, ExecutionEvent{
		Type:      "run.completed",
		RunID:     runID,
		Message:   "run completed",
		Timestamp: e.now(),
	})

	return &RunResult{
		Status: RunStatusCompleted,
		Vars:   scope.Snapshot(),
		Events: collector.Events,
	}, nil
}

func (e *Engine) runSteps(ctx context.Context, req RunRequest, runID string, sink EventSink, scope *Scope, steps []StepDef) error {
	for _, step := range steps {
		if step.ID == "" {
			return fmt.Errorf("%w: step id is required", ErrInvalidWorkflow)
		}
		if err := e.runStep(ctx, req, runID, sink, scope, step); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) runStep(ctx context.Context, req RunRequest, runID string, sink EventSink, scope *Scope, step StepDef) error {
	e.emit(sink, ExecutionEvent{
		Type:      "step.started",
		RunID:     runID,
		StepID:    step.ID,
		Message:   "step started",
		Timestamp: e.now(),
	})

	conditionResult := true
	if step.If != "" {
		result, err := e.Evaluator.EvalBool(step.If, scope)
		if err != nil {
			wrapped := &StepError{StepID: step.ID, Err: err}
			e.emit(sink, ExecutionEvent{
				Type:      "step.failed",
				RunID:     runID,
				StepID:    step.ID,
				Message:   wrapped.Error(),
				Timestamp: e.now(),
			})
			return wrapped
		}
		conditionResult = result
	}

	if !conditionResult {
		e.emit(sink, ExecutionEvent{
			Type:      "step.skipped",
			RunID:     runID,
			StepID:    step.ID,
			Message:   "step condition evaluated to false",
			Timestamp: e.now(),
		})
		if len(step.Else) > 0 {
			if err := e.runSteps(ctx, req, runID, sink, scope, step.Else); err != nil {
				return err
			}
		}
		return nil
	}

	if step.Do != "" {
		op, ok := e.Ops.GetOperation(step.Do)
		if !ok {
			err := fmt.Errorf("%w: %s", ErrUnknownOperation, step.Do)
			wrapped := &StepError{StepID: step.ID, Err: err}
			e.emit(sink, ExecutionEvent{
				Type:      "step.failed",
				RunID:     runID,
				StepID:    step.ID,
				Message:   wrapped.Error(),
				Timestamp: e.now(),
			})
			return wrapped
		}

		resolved, err := e.Resolver.Resolve(step.With, scope)
		if err != nil {
			wrapped := &StepError{StepID: step.ID, Err: err}
			e.emit(sink, ExecutionEvent{
				Type:      "step.failed",
				RunID:     runID,
				StepID:    step.ID,
				Message:   wrapped.Error(),
				Timestamp: e.now(),
			})
			return wrapped
		}

		inputMap, _ := resolved.(map[string]any)
		e.emit(sink, ExecutionEvent{
			Type:      "operation.started",
			RunID:     runID,
			StepID:    step.ID,
			Message:   "operation started",
			Timestamp: e.now(),
			Data:      map[string]any{"operation": step.Do},
		})
		result, err := op.Run(ctx, inputMap, OperationMeta{
			RunID:    runID,
			Workflow: req.Workflow.Metadata.Name,
			StepID:   step.ID,
			Mode:     req.Mode,
		})
		if err != nil {
			wrapped := &StepError{StepID: step.ID, Err: err}
			e.emit(sink, ExecutionEvent{
				Type:      "step.failed",
				RunID:     runID,
				StepID:    step.ID,
				Message:   wrapped.Error(),
				Timestamp: e.now(),
			})
			return wrapped
		}
		e.emit(sink, ExecutionEvent{
			Type:      "operation.completed",
			RunID:     runID,
			StepID:    step.ID,
			Message:   "operation completed",
			Timestamp: e.now(),
			Data:      map[string]any{"operation": step.Do},
		})

		if step.As != "" {
			scope.Set(step.As, cloneMap(result.Output))
		}
	}

	if len(step.Then) > 0 {
		if err := e.runSteps(ctx, req, runID, sink, scope, step.Then); err != nil {
			return err
		}
	}

	e.emit(sink, ExecutionEvent{
		Type:      "step.completed",
		RunID:     runID,
		StepID:    step.ID,
		Message:   "step completed",
		Timestamp: e.now(),
	})
	return nil
}

func (e *Engine) emit(sink EventSink, event ExecutionEvent) {
	if sink != nil {
		sink.Emit(event)
	}
}
