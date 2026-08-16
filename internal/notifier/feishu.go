package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ssh-alertd/internal/event"
)

// Feishu / Lark custom-bot message types.
const (
	feishuMsgText        = "text"
	feishuMsgInteractive = "interactive"
)

// FeishuOptions configures a Feishu (飞书) / Lark custom group robot.
type FeishuOptions struct {
	// WebhookURL is the full custom-bot URL shown when the robot is added to a
	// group (open.feishu.cn for Feishu, open.larksuite.com for Lark).
	WebhookURL string
	// Secret enables signed requests ("签名校验"). Empty means the robot relies on
	// keyword or IP allowlist security instead.
	Secret string
	// MsgType is "text" (default) or "interactive" (message card).
	MsgType string
	// MessageTemplate is an optional Go template for the message. For "text" it
	// renders the message body; for "interactive" it must render a JSON card
	// object. Empty uses the built-in plain-text format.
	MessageTemplate string
}

// Feishu delivers alerts to a Feishu/Lark group through a custom bot webhook.
type Feishu struct {
	url     string
	secret  string
	msgType string
	client  *http.Client

	render renderFunc
	now    func() time.Time
}

// NewFeishu builds a Feishu notifier and compiles any custom template up front
// so template errors surface at startup rather than on the first login.
func NewFeishu(o FeishuOptions) (*Feishu, error) {
	if o.WebhookURL == "" {
		return nil, errors.New("webhook_url is empty")
	}

	msgType := o.MsgType
	if msgType == "" {
		msgType = feishuMsgText
	}
	switch msgType {
	case feishuMsgText, feishuMsgInteractive:
	default:
		return nil, fmt.Errorf("invalid msg_type %q (want text or interactive)", msgType)
	}
	if msgType == feishuMsgInteractive && o.MessageTemplate == "" {
		return nil, errors.New(`msg_type "interactive" requires a message template rendering a JSON card`)
	}

	render, err := buildTextRenderer("feishu", o.MessageTemplate, plainBody)
	if err != nil {
		return nil, fmt.Errorf("message_template: %w", err)
	}

	return &Feishu{
		url:     o.WebhookURL,
		secret:  o.Secret,
		msgType: msgType,
		client:  &http.Client{Timeout: webhookTimeout},
		render:  render,
		now:     time.Now,
	}, nil
}

// Name implements Notifier.
func (f *Feishu) Name() string { return "feishu" }

// Send implements Notifier by POSTing the rendered message to the bot webhook.
func (f *Feishu) Send(ctx context.Context, e event.LoginEvent) error {
	text, err := f.render(e)
	if err != nil {
		return fmt.Errorf("render message: %w", err)
	}

	payload := map[string]any{"msg_type": f.msgType}
	if f.msgType == feishuMsgInteractive {
		// The template renders the card itself; validate it here so a broken
		// card is reported as a template problem rather than an opaque API error.
		var card json.RawMessage
		if err := json.Unmarshal([]byte(text), &card); err != nil {
			return fmt.Errorf("interactive card is not valid JSON: %w", err)
		}
		payload["card"] = card
	} else {
		payload["content"] = map[string]string{"text": text}
	}

	if f.secret != "" {
		ts := strconv.FormatInt(f.now().Unix(), 10)
		payload["timestamp"] = ts
		payload["sign"] = feishuSign(ts, f.secret)
	}

	data, err := postWebhookJSON(ctx, f.client, f.url, payload)
	if err != nil {
		return err
	}
	return checkFeishuResponse(data)
}

// feishuSign implements Feishu's signature scheme, which is unusual: the string
// "timestamp\nsecret" is the HMAC-SHA256 *key* and the message is empty; the
// digest is then base64-encoded. The timestamp is in seconds and must be within
// one hour of the server's clock.
func feishuSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(timestamp+"\n"+secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// feishuResponse covers both the current custom-bot reply (code/msg) and the
// legacy one (StatusCode/StatusMessage), which are still returned by some
// endpoints.
type feishuResponse struct {
	Code          int    `json:"code"`
	Msg           string `json:"msg"`
	StatusCode    int    `json:"StatusCode"`
	StatusMessage string `json:"StatusMessage"`
}

func checkFeishuResponse(data []byte) error {
	var r feishuResponse
	if err := json.Unmarshal(data, &r); err != nil {
		return fmt.Errorf("decode feishu response: %w (body: %s)", err,
			strings.TrimSpace(string(data)))
	}
	if r.Code != 0 {
		return fmt.Errorf("feishu api code %d: %s", r.Code, r.Msg)
	}
	if r.StatusCode != 0 {
		return fmt.Errorf("feishu api code %d: %s", r.StatusCode, r.StatusMessage)
	}
	return nil
}
