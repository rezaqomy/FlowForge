package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"flowforge/internal/kernel"
	"flowforge/internal/secrets"
)

type PollerOptions struct {
	SecretResolver SecretResolver
	BotTokenRef    *secrets.SecretRef
	ProxyURLRef    *secrets.SecretRef
	APIBaseURL     string
	HTTPClient     httpClient
	PollTimeout    time.Duration
	RetryInterval  time.Duration
	Logger         *log.Logger
	HandleEvent    func(context.Context, kernel.Event) error
}

type Poller struct {
	options PollerOptions
	offset  int64
}

type getUpdatesRequest struct {
	Offset         int64    `json:"offset,omitempty"`
	Timeout        int      `json:"timeout"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

type getUpdatesResponse struct {
	OK          bool             `json:"ok"`
	Description string           `json:"description,omitempty"`
	Result      []telegramUpdate `json:"result,omitempty"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message,omitempty"`
}

type telegramMessage struct {
	MessageID int64         `json:"message_id"`
	From      *telegramUser `json:"from,omitempty"`
	Chat      telegramChat  `json:"chat"`
	Text      string        `json:"text,omitempty"`
}

type telegramUser struct {
	ID int64 `json:"id"`
}

type telegramChat struct {
	ID int64 `json:"id"`
}

func NewPoller(options PollerOptions) *Poller {
	if options.APIBaseURL == "" {
		options.APIBaseURL = defaultAPIBaseURL
	}
	if options.PollTimeout == 0 {
		options.PollTimeout = 25 * time.Second
	}
	if options.RetryInterval == 0 {
		options.RetryInterval = 5 * time.Second
	}
	return &Poller{options: options}
}

func (p *Poller) Run(ctx context.Context) {
	var lastError string
	for {
		if err := p.pollOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			message := err.Error()
			if message != lastError {
				p.logf("telegram polling paused: %s", message)
				lastError = message
			}
			if !sleep(ctx, p.options.RetryInterval) {
				return
			}
			continue
		}
		lastError = ""
	}
}

func (p *Poller) pollOnce(ctx context.Context) error {
	token, err := p.botToken(ctx)
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("%s is required for telegram polling", tokenEnvVar)
	}

	client := p.options.HTTPClient
	if client == nil {
		proxy, err := p.proxyURL(ctx)
		if err != nil {
			return err
		}
		client, err = newHTTPClient(proxy)
		if err != nil {
			return err
		}
	}

	updates, err := p.getUpdates(ctx, client, token)
	if err != nil {
		return err
	}
	for _, update := range updates {
		if update.UpdateID >= p.offset {
			p.offset = update.UpdateID + 1
		}
		event, ok := telegramUpdateEvent(update)
		if !ok {
			continue
		}
		if p.options.HandleEvent == nil {
			return fmt.Errorf("telegram polling event handler is not configured")
		}
		if err := p.options.HandleEvent(ctx, event); err != nil {
			p.logf("telegram update %d workflow error: %s", update.UpdateID, err)
		}
	}
	return nil
}

func (p *Poller) getUpdates(ctx context.Context, client httpClient, token string) ([]telegramUpdate, error) {
	body, err := json.Marshal(getUpdatesRequest{
		Offset:         p.offset,
		Timeout:        int(p.options.PollTimeout / time.Second),
		AllowedUpdates: []string{"message"},
	})
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(p.options.APIBaseURL, "/")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/bot%s/getUpdates", baseURL, token), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates request failed: %s", redactToken(err.Error(), token))
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("telegram getUpdates response read failed: %w", err)
	}
	var decoded getUpdatesResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return nil, fmt.Errorf("telegram getUpdates returned invalid JSON: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices || !decoded.OK {
		description := strings.TrimSpace(decoded.Description)
		if description == "" {
			description = resp.Status
		}
		return nil, fmt.Errorf("telegram getUpdates failed: %s", description)
	}
	return decoded.Result, nil
}

func (p *Poller) botToken(ctx context.Context) (string, error) {
	send := SendOperation{
		BotTokenRef:    p.options.BotTokenRef,
		SecretResolver: p.options.SecretResolver,
	}
	return send.botToken(ctx)
}

func (p *Poller) proxyURL(ctx context.Context) (string, error) {
	send := SendOperation{
		ProxyURLRef:    p.options.ProxyURLRef,
		SecretResolver: p.options.SecretResolver,
	}
	return send.proxyURL(ctx)
}

func (p *Poller) logf(format string, args ...any) {
	if p.options.Logger != nil {
		p.options.Logger.Printf(format, args...)
	}
}

func sleep(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func telegramUpdateEvent(update telegramUpdate) (kernel.Event, bool) {
	if update.Message == nil || update.Message.Text == "" {
		return kernel.Event{}, false
	}
	senderID := ""
	if update.Message.From != nil {
		senderID = strconv.FormatInt(update.Message.From.ID, 10)
	}
	return kernel.Event{
		Type: "telegram.message",
		Payload: map[string]any{
			"id":        strconv.FormatInt(update.Message.MessageID, 10),
			"sender_id": senderID,
			"chat_id":   strconv.FormatInt(update.Message.Chat.ID, 10),
			"text":      update.Message.Text,
		},
	}, true
}
