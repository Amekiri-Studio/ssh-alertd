package notifier

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"ssh-alertd/internal/event"
)

// WeCom group-robot message types.
const (
	weComMsgText     = "text"
	weComMsgMarkdown = "markdown"
)

// WeComOptions configures a WeCom (企业微信) group robot.
type WeComOptions struct {
	// WebhookURL is the full robot URL including the key query parameter, as
	// shown when the robot is added to a group. WeCom robots have no signature
	// scheme — the key is the credential, so keep it secret.
	WebhookURL string
	// MsgType is "text" (default) or "markdown".
	MsgType string
	// MessageTemplate is an optional Go template for the message body. Empty
	// uses the built-in plain-text (text) or Markdown (markdown) format.
	MessageTemplate string
	// MentionedList are user IDs to @ in the group ("@all" mentions everyone).
	// Only supported for MsgType "text".
	MentionedList []string
	// MentionedMobileList are phone numbers to @ in the group. Only supported
	// for MsgType "text".
	MentionedMobileList []string
}

// WeCom delivers alerts to a WeCom group through a group robot webhook.
type WeCom struct {
	url        string
	msgType    string
	mentions   []string
	mobiles    []string
	client     *http.Client
	hasMention bool

	render renderFunc
}

// NewWeCom builds a WeCom notifier and compiles any custom template up front so
// template errors surface at startup rather than on the first login.
func NewWeCom(o WeComOptions) (*WeCom, error) {
	if o.WebhookURL == "" {
		return nil, errors.New("webhook_url is empty")
	}

	msgType := o.MsgType
	if msgType == "" {
		msgType = weComMsgText
	}
	def := plainBody
	switch msgType {
	case weComMsgText:
	case weComMsgMarkdown:
		def = markdownBody
	default:
		return nil, fmt.Errorf("invalid msg_type %q (want text or markdown)", msgType)
	}

	hasMention := len(o.MentionedList) > 0 || len(o.MentionedMobileList) > 0
	if hasMention && msgType != weComMsgText {
		return nil, fmt.Errorf("mentioned_list/mentioned_mobile_list require msg_type %q", weComMsgText)
	}

	render, err := buildTextRenderer("wecom", o.MessageTemplate, def)
	if err != nil {
		return nil, fmt.Errorf("message_template: %w", err)
	}

	return &WeCom{
		url:        o.WebhookURL,
		msgType:    msgType,
		mentions:   o.MentionedList,
		mobiles:    o.MentionedMobileList,
		client:     &http.Client{Timeout: webhookTimeout},
		hasMention: hasMention,
		render:     render,
	}, nil
}

// Name implements Notifier.
func (w *WeCom) Name() string { return "wecom" }

// Send implements Notifier by POSTing the rendered message to the robot webhook.
func (w *WeCom) Send(ctx context.Context, e event.LoginEvent) error {
	text, err := w.render(e)
	if err != nil {
		return fmt.Errorf("render message: %w", err)
	}

	payload := map[string]any{"msgtype": w.msgType}
	if w.msgType == weComMsgMarkdown {
		payload["markdown"] = map[string]string{"content": text}
	} else {
		content := map[string]any{"content": text}
		if len(w.mentions) > 0 {
			content["mentioned_list"] = w.mentions
		}
		if len(w.mobiles) > 0 {
			content["mentioned_mobile_list"] = w.mobiles
		}
		payload["text"] = content
	}

	data, err := postWebhookJSON(ctx, w.client, w.url, payload)
	if err != nil {
		return err
	}
	return checkWebhookStatus("wecom", data)
}
