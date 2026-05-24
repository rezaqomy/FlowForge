package app

import "flowforge/internal/kernel"

type WorkflowService struct{}

func (WorkflowService) Normalize(workflow kernel.WorkflowResource) kernel.WorkflowResource {
	return workflow
}
