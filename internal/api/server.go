package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"flowforge/internal/app"
	"flowforge/internal/kernel"
	"flowforge/internal/secrets"
	"flowforge/internal/store"
)

type Server struct {
	secrets   secrets.Store
	workflows store.WorkflowStore
	runs      *app.RunService
}

func NewServer(secretStore secrets.Store, workflowStore store.WorkflowStore, runService *app.RunService) *Server {
	return &Server{
		secrets:   secretStore,
		workflows: workflowStore,
		runs:      runService,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("POST /v1/secrets", s.handleCreateSecret)
	mux.HandleFunc("GET /v1/secrets/", s.handleGetSecret)
	mux.HandleFunc("PUT /v1/secrets/", s.handleUpdateSecret)
	mux.HandleFunc("DELETE /v1/secrets/", s.handleDeleteSecret)
	mux.HandleFunc("POST /v1/workflows", s.handleSaveWorkflow)
	mux.HandleFunc("GET /v1/workflows", s.handleListWorkflows)
	mux.HandleFunc("POST /webhooks/telegram", s.handleTelegramWebhook)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func decodeResource(r *http.Request, out any) error {
	defer r.Body.Close()
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if contentType != "application/json" {
		return unsupportedMediaTypeError{want: "application/json"}
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

type unsupportedMediaTypeError struct {
	want string
}

func (e unsupportedMediaTypeError) Error() string {
	return "unsupported media type: expected " + e.want
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func secretNameFromPath(path string) (string, error) {
	name := strings.TrimPrefix(path, "/v1/secrets/")
	if name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("secret name is required")
	}
	return name, nil
}

func workflowSummary(workflow kernel.WorkflowResource) map[string]any {
	return map[string]any{
		"name":    workflow.Metadata.Name,
		"labels":  workflow.Metadata.Labels,
		"trigger": workflow.Spec.Trigger.Type,
	}
}

func secretView(secret secrets.SecretResource) map[string]any {
	keys := make([]string, 0, len(secret.Data))
	for key := range secret.Data {
		keys = append(keys, key)
	}
	return map[string]any{
		"apiVersion": secret.APIVersion,
		"kind":       secret.Kind,
		"metadata":   secret.Metadata,
		"type":       secret.Type,
		"immutable":  secret.Immutable,
		"dataKeys":   keys,
	}
}

func secretStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, secrets.ErrSecretNotFound):
		return http.StatusNotFound
	case errors.Is(err, secrets.ErrSecretExists):
		return http.StatusConflict
	case errors.Is(err, secrets.ErrInvalidSecret), errors.Is(err, secrets.ErrSecretImmutable):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
