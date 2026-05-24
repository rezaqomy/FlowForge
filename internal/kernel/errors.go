package kernel

import (
	"errors"
	"fmt"
)

var (
	ErrPathNotFound      = errors.New("path not found")
	ErrUnknownOperation  = errors.New("unknown operation")
	ErrInvalidWorkflow   = errors.New("invalid workflow")
	ErrInvalidExpression = errors.New("invalid expression")
)

type StepError struct {
	StepID string
	Err    error
}

func (e *StepError) Error() string {
	return fmt.Sprintf("step %s: %v", e.StepID, e.Err)
}

func (e *StepError) Unwrap() error {
	return e.Err
}
