package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"flowforge/internal/kernel"
	"flowforge/internal/store"
)

func TestWorkflowResourceLifecycle(t *testing.T) {
	server := NewServer(nil, store.NewMemoryWorkflowStore(), nil).Handler()
	workflow := kernel.WorkflowResource{
		APIVersion: "flowforge/v1alpha1",
		Kind:       "Workflow",
		Metadata:   kernel.Metadata{Name: "notify"},
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "telegram.message", As: "message"},
		},
	}

	assertWorkflowRequestStatus(t, server, http.MethodPost, "/v1/workflows", workflow, http.StatusCreated)
	assertWorkflowRequestStatus(t, server, http.MethodPost, "/v1/workflows", workflow, http.StatusConflict)

	workflow.Spec.Trigger.Type = "message.received"
	assertWorkflowRequestStatus(t, server, http.MethodPut, "/v1/workflows/notify", workflow, http.StatusOK)

	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/workflows/notify", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
	}
	var got kernel.WorkflowResource
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	if got.Spec.Trigger.Type != "message.received" {
		t.Fatalf("GET trigger = %q, want message.received", got.Spec.Trigger.Type)
	}

	assertWorkflowRequestStatus(t, server, http.MethodDelete, "/v1/workflows/notify", nil, http.StatusNoContent)
	assertWorkflowRequestStatus(t, server, http.MethodDelete, "/v1/workflows/notify", nil, http.StatusNotFound)
}

func TestUpdateWorkflowRejectsMismatchedName(t *testing.T) {
	server := NewServer(nil, store.NewMemoryWorkflowStore(), nil).Handler()
	workflow := kernel.WorkflowResource{
		Metadata: kernel.Metadata{Name: "other"},
		Spec: kernel.WorkflowSpec{
			Trigger: kernel.TriggerDef{Type: "event", As: "event"},
		},
	}
	assertWorkflowRequestStatus(t, server, http.MethodPut, "/v1/workflows/notify", workflow, http.StatusBadRequest)
}

func assertWorkflowRequestStatus(t *testing.T, handler http.Handler, method, path string, body any, want int) {
	t.Helper()
	var requestBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&requestBody).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	request := httptest.NewRequest(method, path, &requestBody)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("%s %s status = %d, want %d: %s", method, path, response.Code, want, response.Body.String())
	}
}
