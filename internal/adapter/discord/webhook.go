package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"bot-viethoang/internal/domain/model"
	"bot-viethoang/internal/domain/ports"
)

// Webhook is a Discord webhook notifier.
type Webhook struct {
	webhookURL string
	httpClient *http.Client
	logger     ports.Logger
}

// NewWebhook creates a new Discord webhook notifier.
func NewWebhook(webhookURL string, timeout time.Duration, logger ports.Logger) *Webhook {
	return &Webhook{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}
}

// Send posts the notification to Discord.
func (w *Webhook) Send(ctx context.Context, notification model.Notification) error {
	if w.webhookURL == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	payload := map[string]any{
		"content": "",
		"embeds": []map[string]any{
			{
				"title":       truncate(notification.Title, 256),
				"description": truncate(notification.Description, 4096), // Discord supports up to 4096 for description
				"fields":      convertFields(notification.Fields),
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
				"color":       0x5865F2, // Discord blurple color
				"footer": map[string]string{
					"text": "🤖 Daily Bot • Powered by Thomas",
				},
				"author": map[string]string{
					"name":     "Daily Bot",
					"icon_url": "https://cdn.discordapp.com/emojis/1234567890123456789.png",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	w.logger.Info(ctx, "notification sent to discord")
	return nil
}

func convertFields(fields []model.NotificationField) []map[string]any {
	if len(fields) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(fields))
	for _, field := range fields {
		result = append(result, map[string]any{
			"name":   truncate(field.Name, 256),
			"value":  truncate(field.Value, 1024),
			"inline": field.Inline,
		})
	}

	return result
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}
