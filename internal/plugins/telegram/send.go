package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"flowforge/internal/kernel"
	"flowforge/internal/secrets"
)

const (
	defaultAPIBaseURL = "https://api.telegram.org"
	defaultTimeout    = 10 * time.Second
	proxyEnvVar       = "TELEGRAM_PROXY_URL"
	tokenEnvVar       = "TELEGRAM_BOT_TOKEN"
)

type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type SecretResolver interface {
	Resolve(ctx context.Context, ref secrets.SecretRef) ([]byte, error)
}

type SendOptions struct {
	SecretResolver SecretResolver
	BotTokenRef    *secrets.SecretRef
	ProxyURLRef    *secrets.SecretRef
}

type SendOperation struct {
	BotToken       string
	BotTokenRef    *secrets.SecretRef
	APIBaseURL     string
	HTTPClient     httpClient
	ProxyURL       string
	ProxyURLRef    *secrets.SecretRef
	SecretResolver SecretResolver
}

func NewSendOperation() *SendOperation {
	return NewSendOperationWithOptions(SendOptions{})
}

func NewSendOperationWithOptions(options SendOptions) *SendOperation {
	return &SendOperation{
		APIBaseURL:     defaultAPIBaseURL,
		SecretResolver: options.SecretResolver,
		BotTokenRef:    options.BotTokenRef,
		ProxyURLRef:    options.ProxyURLRef,
	}
}

func (s *SendOperation) Run(ctx context.Context, input map[string]any, meta kernel.OperationMeta) (kernel.OperationResult, error) {
	to, ok := input["to"].(string)
	if !ok || strings.TrimSpace(to) == "" {
		return kernel.OperationResult{}, fmt.Errorf("to must be a non-empty string")
	}
	text, ok := input["text"].(string)
	if !ok || strings.TrimSpace(text) == "" {
		return kernel.OperationResult{}, fmt.Errorf("text must be a non-empty string")
	}

	parseMode, err := optionalString(input, "parse_mode")
	if err != nil {
		return kernel.OperationResult{}, err
	}

	disableNotification, err := optionalBool(input, "disable_notification")
	if err != nil {
		return kernel.OperationResult{}, err
	}

	if meta.Mode != kernel.RunModeLive {
		return kernel.OperationResult{
			Output: map[string]any{
				"message_id": "dryrun-msg-1",
			},
		}, nil
	}

	messageID, err := s.sendMessage(ctx, sendMessageRequest{
		ChatID:              to,
		Text:                text,
		ParseMode:           parseMode,
		DisableNotification: disableNotification,
	})
	if err != nil {
		return kernel.OperationResult{}, err
	}

	return kernel.OperationResult{
		Output: map[string]any{
			"message_id": messageID,
		},
	}, nil
}

func optionalString(input map[string]any, key string) (string, error) {
	value, ok := input[key]
	if !ok || value == nil {
		return "", nil
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return text, nil
}

func optionalBool(input map[string]any, key string) (bool, error) {
	value, ok := input[key]
	if !ok || value == nil {
		return false, nil
	}
	flag, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return flag, nil
}

type sendMessageRequest struct {
	ChatID              string `json:"chat_id"`
	Text                string `json:"text"`
	ParseMode           string `json:"parse_mode,omitempty"`
	DisableNotification bool   `json:"disable_notification,omitempty"`
}

type sendMessageResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      struct {
		MessageID int64 `json:"message_id"`
	} `json:"result,omitempty"`
}

func (s *SendOperation) sendMessage(ctx context.Context, payload sendMessageRequest) (string, error) {
	token, err := s.botToken(ctx)
	if err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("%s is required for live telegram.send", tokenEnvVar)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := s.HTTPClient
	if client == nil {
		proxy, err := s.proxyURL(ctx)
		if err != nil {
			return "", err
		}
		client, err = newHTTPClient(proxy)
		if err != nil {
			return "", err
		}
	}
	baseURL := strings.TrimRight(s.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultAPIBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/bot%s/sendMessage", baseURL, token), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram sendMessage request failed: %s", redactToken(err.Error(), token))
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("telegram sendMessage response read failed: %w", err)
	}

	var decoded sendMessageResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return "", fmt.Errorf("telegram sendMessage returned invalid JSON: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !decoded.OK {
		description := strings.TrimSpace(decoded.Description)
		if description == "" {
			description = resp.Status
		}
		return "", fmt.Errorf("telegram sendMessage failed: %s", description)
	}
	if decoded.Result.MessageID == 0 {
		return "", fmt.Errorf("telegram sendMessage response missing message_id")
	}

	return fmt.Sprintf("%d", decoded.Result.MessageID), nil
}

func (s *SendOperation) botToken(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.BotToken) != "" {
		return strings.TrimSpace(s.BotToken), nil
	}
	if s.BotTokenRef != nil {
		return s.resolveSecretString(ctx, *s.BotTokenRef, "telegram bot token")
	}
	return strings.TrimSpace(os.Getenv(tokenEnvVar)), nil
}

func (s *SendOperation) proxyURL(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.ProxyURL) != "" {
		return strings.TrimSpace(s.ProxyURL), nil
	}
	if s.ProxyURLRef != nil {
		value, err := s.resolveSecretString(ctx, *s.ProxyURLRef, "telegram proxy URL")
		if err != nil {
			if errors.Is(err, secrets.ErrSecretNotFound) {
				return "", nil
			}
			return "", err
		}
		return value, nil
	}
	return strings.TrimSpace(os.Getenv(proxyEnvVar)), nil
}

func (s *SendOperation) resolveSecretString(ctx context.Context, ref secrets.SecretRef, label string) (string, error) {
	if s.SecretResolver == nil {
		return "", fmt.Errorf("%s secret resolver is not configured", label)
	}
	value, err := s.SecretResolver.Resolve(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("resolve %s secret %s/%s: %w", label, ref.Name, ref.Key, err)
	}
	trimmed := strings.TrimSpace(string(value))
	if trimmed == "" {
		return "", fmt.Errorf("%s secret %s/%s is empty", label, ref.Name, ref.Key)
	}
	return trimmed, nil
}

func redactToken(message, token string) string {
	if token == "" {
		return message
	}
	return strings.ReplaceAll(message, token, "<redacted>")
}

func newHTTPClient(proxy string) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if proxy != "" {
		parsed, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("%s must be a valid URL: %w", proxyEnvVar, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "socks5" {
			return nil, fmt.Errorf("%s must use http, https, or socks5 scheme", proxyEnvVar)
		}
		transport.Proxy = http.ProxyURL(parsed)
	}

	return &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}, nil
}
