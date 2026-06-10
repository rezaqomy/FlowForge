package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
	"flowforge/internal/secrets"
)

func TestDryRunDoesNotTriggerRealSideEffect(t *testing.T) {
	reg := kernel.NewRegistry()
	cat := catalog.New()
	Register(reg, cat)

	op, ok := reg.GetOperation("telegram.send")
	if !ok {
		t.Fatalf("expected telegram.send operation")
	}
	send, ok := op.(*SendOperation)
	if !ok {
		t.Fatalf("unexpected operation type %T", op)
	}
	called := false
	send.BotToken = "token"
	send.APIBaseURL = "https://telegram.invalid"
	send.HTTPClient = roundTripFunc(func(*http.Request) (*http.Response, error) {
		called = true
		return nil, nil
	})
	manifest, ok := cat.GetOperationManifest("telegram.send")
	if !ok {
		t.Fatalf("expected telegram.send manifest")
	}
	if !manifest.SideEffect {
		t.Fatalf("telegram.send should be marked as side-effecting")
	}

	result, err := send.Run(context.Background(), map[string]any{
		"to":   "admin",
		"text": "hello",
	}, kernel.OperationMeta{Mode: kernel.RunModeDryRun})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if called {
		t.Fatalf("dry-run should not call Telegram")
	}
	if _, ok := result.Output["message_id"].(string); !ok {
		t.Fatalf("expected message_id output")
	}
}

func TestLiveModeSendsTelegramMessage(t *testing.T) {
	var requestPath string
	var requestPayload map[string]any
	client := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
			t.Fatalf("Content-Type = %s, want application/json", contentType)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":42}}`)),
			Header:     make(http.Header),
		}, nil
	})

	send := &SendOperation{
		BotToken:   "test-token",
		APIBaseURL: "https://telegram.test",
		HTTPClient: client,
	}
	result, err := send.Run(context.Background(), map[string]any{
		"to":                   "12345",
		"text":                 "hello",
		"parse_mode":           "HTML",
		"disable_notification": true,
	}, kernel.OperationMeta{Mode: kernel.RunModeLive})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if requestPath != "/bottest-token/sendMessage" {
		t.Fatalf("path = %s, want /bottest-token/sendMessage", requestPath)
	}
	if requestPayload["chat_id"] != "12345" {
		t.Fatalf("chat_id = %v, want 12345", requestPayload["chat_id"])
	}
	if requestPayload["text"] != "hello" {
		t.Fatalf("text = %v, want hello", requestPayload["text"])
	}
	if requestPayload["parse_mode"] != "HTML" {
		t.Fatalf("parse_mode = %v, want HTML", requestPayload["parse_mode"])
	}
	if requestPayload["disable_notification"] != true {
		t.Fatalf("disable_notification = %v, want true", requestPayload["disable_notification"])
	}
	if result.Output["message_id"] != "42" {
		t.Fatalf("message_id = %v, want 42", result.Output["message_id"])
	}
}

func TestLiveModeReturnsTelegramError(t *testing.T) {
	send := &SendOperation{
		BotToken:   "test-token",
		APIBaseURL: "https://telegram.test",
		HTTPClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Status:     "400 Bad Request",
				Body:       io.NopCloser(strings.NewReader(`{"ok":false,"description":"Bad Request: chat not found"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	_, err := send.Run(context.Background(), map[string]any{
		"to":   "missing-chat",
		"text": "hello",
	}, kernel.OperationMeta{Mode: kernel.RunModeLive})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLiveModeRequiresBotToken(t *testing.T) {
	t.Setenv(tokenEnvVar, "")
	send := &SendOperation{}

	_, err := send.Run(context.Background(), map[string]any{
		"to":   "12345",
		"text": "hello",
	}, kernel.OperationMeta{Mode: kernel.RunModeLive})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestLiveModeRedactsBotTokenFromRequestErrors(t *testing.T) {
	send := &SendOperation{
		BotToken:   "secret-token",
		APIBaseURL: "https://telegram.test",
		HTTPClient: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("Post https://telegram.test/botsecret-token/sendMessage: dial failed")
		}),
	}

	_, err := send.Run(context.Background(), map[string]any{
		"to":   "12345",
		"text": "hello",
	}, kernel.OperationMeta{Mode: kernel.RunModeLive})
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), "secret-token") {
		t.Fatalf("error leaked bot token: %v", err)
	}
	if !strings.Contains(err.Error(), "<redacted>") {
		t.Fatalf("error should contain redaction marker: %v", err)
	}
}

func TestProxyURLPrefersConfiguredValueOverEnvironment(t *testing.T) {
	t.Setenv(proxyEnvVar, "http://env-proxy.example:8080")

	got, err := (&SendOperation{ProxyURL: "socks5://configured-proxy.example:1080"}).proxyURL(context.Background())
	if err != nil {
		t.Fatalf("proxyURL() error = %v", err)
	}
	if got != "socks5://configured-proxy.example:1080" {
		t.Fatalf("proxyURL() = %q, want configured proxy", got)
	}
}

func TestBotTokenCanResolveFromSecretStore(t *testing.T) {
	store := secrets.NewEncryptedMemoryStore(testCipher())
	if err := store.Create(context.Background(), secrets.SecretResource{
		Metadata: secrets.Metadata{Name: "telegram-bot"},
		Type:     secrets.TypeAPIKey,
		Data:     map[string][]byte{"api-key": []byte("secret-token")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	send := NewSendOperationWithOptions(SendOptions{
		SecretResolver: store,
		BotTokenRef:    &secrets.SecretRef{Name: "telegram-bot", Key: "api-key"},
	})
	got, err := send.botToken(context.Background())
	if err != nil {
		t.Fatalf("botToken() error = %v", err)
	}
	if got != "secret-token" {
		t.Fatalf("botToken() = %q, want secret-token", got)
	}
}

func TestProxyURLCanResolveFromSecretStore(t *testing.T) {
	store := secrets.NewEncryptedMemoryStore(testCipher())
	if err := store.Create(context.Background(), secrets.SecretResource{
		Metadata: secrets.Metadata{Name: "telegram-proxy"},
		Data:     map[string][]byte{"url": []byte("socks5://127.0.0.1:1080")},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	send := NewSendOperationWithOptions(SendOptions{
		SecretResolver: store,
		ProxyURLRef:    &secrets.SecretRef{Name: "telegram-proxy", Key: "url"},
	})
	got, err := send.proxyURL(context.Background())
	if err != nil {
		t.Fatalf("proxyURL() error = %v", err)
	}
	if got != "socks5://127.0.0.1:1080" {
		t.Fatalf("proxyURL() = %q, want socks5://127.0.0.1:1080", got)
	}
}

func TestSecretRefRequiresSecretResolver(t *testing.T) {
	send := NewSendOperationWithOptions(SendOptions{
		BotTokenRef: &secrets.SecretRef{Name: "telegram-bot", Key: "api-key"},
	})
	_, err := send.botToken(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewHTTPClientUsesExplicitProxy(t *testing.T) {
	client, err := newHTTPClient("http://proxy.example:8080")
	if err != nil {
		t.Fatalf("newHTTPClient() error = %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.telegram.org/bottest/sendMessage", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	proxy, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy() error = %v", err)
	}
	if proxy.String() != "http://proxy.example:8080" {
		t.Fatalf("proxy = %q, want http://proxy.example:8080", proxy.String())
	}
}

func TestNewHTTPClientRejectsUnsupportedProxyScheme(t *testing.T) {
	_, err := newHTTPClient("ftp://proxy.example:21")
	if err == nil {
		t.Fatalf("expected error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func testCipher() *secrets.EnvelopeCipher {
	key := make([]byte, 32)
	key[0] = 17
	return secrets.NewEnvelopeCipher(secrets.StaticKeyProvider{Key: key})
}
