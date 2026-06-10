package telegram

import (
	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
)

func Register(reg *kernel.Registry, cat *catalog.Registry) {
	RegisterWithOptions(reg, cat, SendOptions{})
}

func RegisterWithOptions(reg *kernel.Registry, cat *catalog.Registry, options SendOptions) {
	reg.Register("telegram.send", NewSendOperationWithOptions(options))
	cat.RegisterManifest(catalog.PluginManifest{
		Name: "telegram",
		Triggers: []catalog.TriggerManifest{
			{
				Type:        "telegram.message",
				Description: "Incoming Telegram message.",
				DefaultAs:   "message",
				EventSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"id":        {Type: "string"},
						"sender_id": {Type: "string"},
						"chat_id":   {Type: "string"},
						"text":      {Type: "string"},
					},
					Required: []string{"id", "sender_id", "chat_id", "text"},
				},
			},
		},
		Operations: []catalog.OperationManifest{
			{
				Type:        "telegram.send",
				Description: "Sends a Telegram message.",
				InputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"to":                   {Type: "string"},
						"text":                 {Type: "string"},
						"parse_mode":           {Type: "string"},
						"disable_notification": {Type: "boolean"},
					},
					Required: []string{"to", "text"},
				},
				OutputSchema: catalog.Schema{
					Type: "object",
					Properties: map[string]catalog.Schema{
						"message_id": {Type: "string"},
					},
					Required: []string{"message_id"},
				},
				SideEffect: true,
				Display:    catalog.DisplayHints{Label: "telegram.send"},
			},
		},
	})
}
