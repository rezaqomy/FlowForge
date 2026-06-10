package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowforge/internal/secrets"
	"flowforge/internal/store"
)

func TestListSecretsRedactsValues(t *testing.T) {
	key := make([]byte, 32)
	key[0] = 7
	secretStore := secrets.NewEncryptedMemoryStore(
		secrets.NewEnvelopeCipher(secrets.StaticKeyProvider{Key: key}),
	)
	if err := secretStore.Create(context.Background(), secrets.SecretResource{
		Metadata: secrets.Metadata{Name: "telegram-proxy"},
		Data:     map[string][]byte{"url": []byte("socks5://user:password@127.0.0.1:9050")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	handler := NewServer(secretStore, store.NewMemoryWorkflowStore(), nil).Handler()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/secrets", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/secrets status = %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "password") || strings.Contains(body, "127.0.0.1") {
		t.Fatalf("GET /v1/secrets exposed secret data: %s", body)
	}
	if !strings.Contains(body, `"dataKeys":["url"]`) {
		t.Fatalf("GET /v1/secrets missing data keys: %s", body)
	}
}
