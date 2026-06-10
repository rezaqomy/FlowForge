package api

import (
	"errors"
	"fmt"
	"net/http"

	"flowforge/internal/kernel"
)

type WorkflowHandler struct{}

func (s *Server) handleSaveWorkflow(w http.ResponseWriter, r *http.Request) {
	var workflow kernel.WorkflowResource
	if err := decodeResource(r, &workflow); err != nil {
		var mediaTypeErr unsupportedMediaTypeError
		if errors.As(err, &mediaTypeErr) {
			writeError(w, http.StatusUnsupportedMediaType, err)
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode workflow: %w", err))
		return
	}
	if workflow.Metadata.Name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow metadata.name is required"))
		return
	}
	if workflow.Spec.Trigger.Type == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow spec.trigger.type is required"))
		return
	}
	if workflow.Spec.Trigger.As == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workflow spec.trigger.as is required"))
		return
	}
	if err := s.workflows.Save(workflow); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, workflowSummary(workflow))
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
