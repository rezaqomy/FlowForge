package api

import (
	"errors"
	"fmt"
	"net/http"

	"flowforge/internal/secrets"
)

type RunHandler struct{}

func (s *Server) handleCreateSecret(w http.ResponseWriter, r *http.Request) {
	var secret secrets.SecretResource
	if err := decodeResource(r, &secret); err != nil {
		var mediaTypeErr unsupportedMediaTypeError
		if errors.As(err, &mediaTypeErr) {
			writeError(w, http.StatusUnsupportedMediaType, err)
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode secret: %w", err))
		return
	}
	if err := s.secrets.Create(r.Context(), secret); err != nil {
		writeError(w, secretStatus(err), err)
		return
	}
	created, err := s.secrets.Get(r.Context(), secret.Normalized().Metadata.Name)
	if err != nil {
		writeError(w, secretStatus(err), err)
		return
	}
	writeJSON(w, http.StatusCreated, secretView(created))
}

func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	name, err := secretNameFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	secret, err := s.secrets.Get(r.Context(), name)
	if err != nil {
		writeError(w, secretStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, secretView(secret))
}

func (s *Server) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	list, err := s.secrets.List(r.Context())
	if err != nil {
		writeError(w, secretStatus(err), err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, secret := range list {
		out = append(out, secretView(secret))
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": out})
}

func (s *Server) handleUpdateSecret(w http.ResponseWriter, r *http.Request) {
	name, err := secretNameFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var secret secrets.SecretResource
	if err := decodeResource(r, &secret); err != nil {
		var mediaTypeErr unsupportedMediaTypeError
		if errors.As(err, &mediaTypeErr) {
			writeError(w, http.StatusUnsupportedMediaType, err)
			return
		}
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode secret: %w", err))
		return
	}
	if secret.Metadata.Name == "" {
		secret.Metadata.Name = name
	}
	if secret.Metadata.Name != name {
		writeError(w, http.StatusBadRequest, fmt.Errorf("secret name in path and body must match"))
		return
	}
	var updateErr error
	if r.URL.Query().Get("replace") == "true" {
		updateErr = s.secrets.Replace(r.Context(), secret)
	} else {
		updateErr = s.secrets.Update(r.Context(), secret)
	}
	if updateErr != nil {
		writeError(w, secretStatus(updateErr), updateErr)
		return
	}
	updated, err := s.secrets.Get(r.Context(), name)
	if err != nil {
		writeError(w, secretStatus(err), err)
		return
	}
	writeJSON(w, http.StatusOK, secretView(updated))
}

func (s *Server) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name, err := secretNameFromPath(r.URL.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.secrets.Delete(r.Context(), name); err != nil {
		writeError(w, secretStatus(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
