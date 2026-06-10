package store

import (
	"errors"

	"flowforge/internal/kernel"
)

var (
	ErrWorkflowNotFound = errors.New("workflow not found")
	ErrWorkflowExists   = errors.New("workflow already exists")
)

type WorkflowStore interface {
	Create(workflow kernel.WorkflowResource) error
	Get(name string) (kernel.WorkflowResource, error)
	Save(workflow kernel.WorkflowResource) error
	Update(workflow kernel.WorkflowResource) error
	Delete(name string) error
	List() ([]kernel.WorkflowResource, error)
}
