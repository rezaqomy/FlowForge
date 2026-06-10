package store

import "flowforge/internal/kernel"

type WorkflowStore interface {
	Save(workflow kernel.WorkflowResource) error
	List() ([]kernel.WorkflowResource, error)
}
