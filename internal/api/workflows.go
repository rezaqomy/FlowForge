package api

import (
	"errors"
	"fmt"
	"net/http"

	"flowforge/internal/kernel"
)

type WorkflowHandler struct{}

func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	workflow, ok := decodeWorkflow(w, r)
	if !ok {
		return
	}
	if err := s.workflows.Create(workflow); err != nil {
		writeError(w, workflowStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, workflowSummary(workflow))
}

func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	name, err := workflowNameFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	workflow, err := s.workflows.Get(name)
	if err != nil {
		writeError(w, workflowStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, workflow)
}

func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	name, err := workflowNameFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	workflow, ok := decodeWorkflow(w, r)
	if !ok {
		return
	}
	if workflow.Metadata.Name != name {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow name in path and body must match"))
		return
	}
	if err := s.workflows.Update(workflow); err != nil {
		writeError(w, workflowStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, workflowSummary(workflow))
}

func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	name, err := workflowNameFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.workflows.Delete(name); err != nil {
		writeError(w, workflowStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeWorkflow(w http.ResponseWriter, r *http.Request) (kernel.WorkflowResource, bool) {
	var workflow kernel.WorkflowResource
	if err := decodeResource(r, &workflow); err != nil {
		var mediaTypeErr unsupportedMediaTypeError
		if errors.As(err, &mediaTypeErr) {
			writeError(w, http.StatusUnsupportedMediaType, err)
			return kernel.WorkflowResource{}, false
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode workflow: %w", err))
		return kernel.WorkflowResource{}, false
	}
	if workflow.Metadata.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow metadata.name is required"))
		return kernel.WorkflowResource{}, false
	}
	if workflow.Spec.Trigger.Type == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow spec.trigger.type is required"))
		return kernel.WorkflowResource{}, false
	}
	if workflow.Spec.Trigger.As == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow spec.trigger.as is required"))
		return kernel.WorkflowResource{}, false
	}
	return workflow, true
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, _ *http.Request) {
	workflows, err := s.workflows.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(workflows))
	for _, workflow := range workflows {
		out = append(out, workflowSummary(workflow))
	}
	writeJSON(w, http.StatusOK, map[string]any{"workflows": out})
}
