package notifier

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"ssh-alertd/internal/event"
)

// DingTalk custom-robot message types.
const (
	dingMsgText     = "text"
	dingMsgMarkdown = "markdown"
)

// DingTalkOptions configures a DingTalk (钉钉) custom group robot.
type DingTalkOptions struct {
	// WebhookURL is the full robot URL including the access_token query
	// parameter, as shown when the robot is created.
	WebhookURL string
	// Secret enables signed requests ("加签"). Empty means the robot relies on
	// keyword or IP allowlist security instead — DingTalk requires at least one.
	Secret string
	// MsgType is "text" (default) or "markdown".
	MsgType string
	// Title is the markdown notification title shown in the conversation list.
	// Only used when MsgType is "markdown"; empty uses a built-in title.
	Title string
	// MessageTemplate is an optional Go template for the message body. Empty
	// uses the built-in plain-text (text) or Markdown (markdown) format.
	MessageTemplate string
}

// DingTalk delivers alerts to a DingTalk group through a custom robot webhook.
type DingTalk struct {
	url     string
	secret  string
	msgType string
	title   string
	client  *http.Client

	render renderFunc
	now    func() time.Time
}

// NewDingTalk builds a DingTalk notifier and compiles any custom template up
// front so template errors surface at startup rather than on the first login.
func NewDingTalk(o DingTalkOptions) (*DingTalk, error) {
	if o.WebhookURL == "" {
		return nil, errors.New("webhook_url is empty")
	}

	msgType := o.MsgType
	if msgType == "" {
		msgType = dingMsgText
	}
	def := plainBody
	switch msgType {
	case dingMsgText:
	case dingMsgMarkdown:
		def = markdownBody
	default:
		return nil, fmt.Errorf("invalid msg_type %q (want text or markdown)", msgType)
	}

	render, err := buildTextRenderer("dingtalk", o.MessageTemplate, def)
	if err != nil {
		return nil, fmt.Errorf("message_template: %w", err)
	}

	title := o.Title
	if title == "" {
		title = "SSH Login Alert"
	}

	return &DingTalk{
		url:     o.WebhookURL,
		secret:  o.Secret,
		msgType: msgType,
		title:   title,
		client:  &http.Client{Timeout: webhookTimeout},
		render:  render,
		now:     time.Now,
	}, nil
}

// Name implements Notifier.
func (d *DingTalk) Name() string { return "dingtalk" }

// Send implements Notifier by POSTing the rendered message to the robot webhook.
func (d *DingTalk) Send(ctx context.Context, e event.LoginEvent) error {
	text, err := d.render(e)
	if err != nil {
		return fmt.Errorf("render message: %w", err)
	}

	payload := map[string]any{"msgtype": d.msgType}
	if d.msgType == dingMsgMarkdown {
		payload["markdown"] = map[string]string{"title": d.title, "text": text}
	} else {
		payload["text"] = map[string]string{"content": text}
	}

	endpoint, err := d.signedURL()
	if err != nil {
		return err
	}

	data, err := postWebhookJSON(ctx, d.client, endpoint, payload)
	if err != nil {
		return err
	}
	return checkWebhookStatus("dingtalk", data)
}

// signedURL appends the timestamp/sign query parameters when a secret is set.
// DingTalk uses millisecond timestamps and requires the request within 1 hour of
// the server clock.
func (d *DingTalk) signedURL() (string, error) {
	if d.secret == "" {
		return d.url, nil
	}

	u, err := url.Parse(d.url)
	if err != nil {
		return "", fmt.Errorf("parse webhook_url: %w", err)
	}
	ts := strconv.FormatInt(d.now().UnixMilli(), 10)

	q := u.Query()
	q.Set("timestamp", ts)
	q.Set("sign", dingTalkSign(ts, d.secret))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// dingTalkSign signs "timestamp\nsecret" with HMAC-SHA256 keyed by the secret,
// base64-encoded. (Unlike Feishu, the secret is the key and the joined string is
// the message.) URL escaping is handled by url.Values.Encode.
func dingTalkSign(timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}
