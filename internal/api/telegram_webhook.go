package api

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"flowforge/internal/kernel"
	"flowforge/internal/secrets"
)

const telegramWebhookSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

var telegramWebhookSecretRef = secrets.SecretRef{Name: "telegram-webhook", Key: "secret-token"}

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

func (s *Server) handleTelegramWebhook(w http.ResponseWriter, r *http.Request) {
	if err := s.verifyTelegramWebhookSecret(r); err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	defer r.Body.Close()
	var update telegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("decode telegram update: %w", err))
		return
	}

	event, ok := telegramUpdateEvent(update)
	if !ok {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":  "ignored",
			"reason":  "telegram update does not contain a text message",
			"update":  update.UpdateID,
			"handled": 0,
		})
		return
	}

	workflows, err := s.workflows.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	results := make([]map[string]any, 0)
	for _, workflow := range workflows {
		if workflow.Spec.Trigger.Type != "telegram.message" {
			continue
		}
		result, err := s.runs.Run(r.Context(), kernel.RunRequest{
			Workflow: workflow,
			Event:    event,
			Inputs:   map[string]any{},
			Mode:     kernel.RunModeLive,
		})
		item := map[string]any{
			"workflow": workflow.Metadata.Name,
		}
		if result != nil && result.Run != nil {
			item["status"] = result.Run.Status
		}
		if err != nil {
			item["error"] = err.Error()
		}
		results = append(results, item)
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":    "accepted",
		"update":    update.UpdateID,
		"trigger":   event.Type,
		"workflows": results,
	})
}

func (s *Server) verifyTelegramWebhookSecret(r *http.Request) error {
	expected, err := s.secrets.Resolve(r.Context(), telegramWebhookSecretRef)
	if err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return fmt.Errorf("telegram webhook secret is not configured")
		}
		return fmt.Errorf("resolve telegram webhook secret: %w", err)
	}
	got := r.Header.Get(telegramWebhookSecretHeader)
	if subtle.ConstantTimeCompare([]byte(got), expected) != 1 {
		return fmt.Errorf("invalid telegram webhook secret")
	}
	return nil
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
