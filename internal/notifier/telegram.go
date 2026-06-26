package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"

	"ssh-alertd/internal/event"
)

// Telegram delivers alerts through the Telegram Bot API (sendMessage).
type Telegram struct {
	botToken string
	chatID   string
	apiBase  string
	client   *http.Client
}

// NewTelegram builds a Telegram notifier. apiBase may be empty, in which case
// the public Telegram endpoint is used.
func NewTelegram(botToken, chatID, apiBase string) *Telegram {
	if apiBase == "" {
		apiBase = "https://api.telegram.org"
	}
	return &Telegram{
		botToken: botToken,
		chatID:   chatID,
		apiBase:  strings.TrimRight(apiBase, "/"),
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Name implements Notifier.
func (t *Telegram) Name() string { return "telegram" }

// sendMessageRequest mirrors the subset of Telegram's sendMessage payload we use.
type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// Send implements Notifier by POSTing an HTML-formatted message.
func (t *Telegram) Send(ctx context.Context, e event.LoginEvent) error {
	payload := sendMessageRequest{
		ChatID:    t.chatID,
		Text:      t.format(e),
		ParseMode: "HTML",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", t.apiBase, t.botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("telegram api status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

// format renders the event as Telegram HTML. User-controlled fields are
// escaped to avoid breaking the markup or injecting tags.
func (t *Telegram) format(e event.LoginEvent) string {
	esc := html.EscapeString
	return fmt.Sprintf(
		"🔐 <b>SSH Login Alert</b>\n\n"+
			"<b>Host:</b> <code>%s</code>\n"+
			"<b>User:</b> <code>%s</code>\n"+
			"<b>From IP:</b> <code>%s</code>\n"+
			"<b>Client Port:</b> <code>%s</code>\n"+
			"<b>Method:</b> <code>%s</code>\n"+
			"<b>Time:</b> <code>%s</code>",
		esc(e.Hostname), esc(e.Username), esc(e.IP), esc(e.Port), esc(e.Method),
		esc(e.Time.Format("2006-01-02 15:04:05 MST")),
	)
}
