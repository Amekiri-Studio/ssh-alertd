package notifier

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestDingTalkTextPayload(t *testing.T) {
	srv, body, query := captureWebhook(t, `{"errcode":0,"errmsg":"ok"}`)

	d, err := NewDingTalk(DingTalkOptions{WebhookURL: srv.URL + "?access_token=abc"})
	if err != nil {
		t.Fatalf("NewDingTalk: %v", err)
	}
	if err := d.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		MsgType string            `json:"msgtype"`
		Text    map[string]string `json:"text"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MsgType != "text" {
		t.Errorf("msgtype = %q, want text", payload.MsgType)
	}
	if !strings.Contains(payload.Text["content"], "alice") {
		t.Errorf("content missing username: %q", payload.Text["content"])
	}
	// Without a secret the original query string must survive untouched.
	if *query != "access_token=abc" {
		t.Errorf("query = %q, want access_token=abc", *query)
	}
}

func TestDingTalkMarkdownPayload(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"errcode":0}`)

	d, err := NewDingTalk(DingTalkOptions{
		WebhookURL: srv.URL,
		MsgType:    "markdown",
		Title:      "Login",
	})
	if err != nil {
		t.Fatalf("NewDingTalk: %v", err)
	}
	if err := d.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		MsgType  string            `json:"msgtype"`
		Markdown map[string]string `json:"markdown"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MsgType != "markdown" {
		t.Errorf("msgtype = %q, want markdown", payload.MsgType)
	}
	if payload.Markdown["title"] != "Login" {
		t.Errorf("title = %q, want Login", payload.Markdown["title"])
	}
	if !strings.Contains(payload.Markdown["text"], "**User:**") {
		t.Errorf("markdown body not used: %q", payload.Markdown["text"])
	}
}

// TestDingTalkSignedURL checks that signing adds timestamp/sign as query
// parameters (DingTalk signs in the URL, not the body) and preserves
// access_token. The timestamp is in milliseconds.
func TestDingTalkSignedURL(t *testing.T) {
	srv, _, query := captureWebhook(t, `{"errcode":0}`)

	d, err := NewDingTalk(DingTalkOptions{
		WebhookURL: srv.URL + "?access_token=abc",
		Secret:     "SECfoo",
	})
	if err != nil {
		t.Fatalf("NewDingTalk: %v", err)
	}
	d.now = func() time.Time { return time.Unix(1700000000, 0) }

	if err := d.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	q, err := url.ParseQuery(*query)
	if err != nil {
		t.Fatalf("parse query: %v", err)
	}
	if q.Get("access_token") != "abc" {
		t.Errorf("access_token lost: %q", *query)
	}
	if q.Get("timestamp") != "1700000000000" {
		t.Errorf("timestamp = %q, want milliseconds 1700000000000", q.Get("timestamp"))
	}
	if want := dingTalkSign("1700000000000", "SECfoo"); q.Get("sign") != want {
		t.Errorf("sign = %q, want %q", q.Get("sign"), want)
	}
}

func TestDingTalkAPIError(t *testing.T) {
	srv, _, _ := captureWebhook(t, `{"errcode":310000,"errmsg":"keywords not in content"}`)

	d, err := NewDingTalk(DingTalkOptions{WebhookURL: srv.URL})
	if err != nil {
		t.Fatalf("NewDingTalk: %v", err)
	}
	err = d.Send(context.Background(), testEvent())
	if err == nil || !strings.Contains(err.Error(), "310000") {
		t.Errorf("expected api error, got %v", err)
	}
}

func TestDingTalkCustomTemplate(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"errcode":0}`)

	d, err := NewDingTalk(DingTalkOptions{
		WebhookURL:      srv.URL,
		MessageTemplate: "SSH: {{.Username}}@{{.Hostname}} from {{.IP}}",
	})
	if err != nil {
		t.Fatalf("NewDingTalk: %v", err)
	}
	if err := d.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(string(*body), "SSH: alice@myhost from 203.0.113.5") {
		t.Errorf("custom template not rendered: %s", *body)
	}
}

func TestDingTalkValidation(t *testing.T) {
	if _, err := NewDingTalk(DingTalkOptions{}); err == nil {
		t.Error("expected error for empty webhook_url")
	}
	if _, err := NewDingTalk(DingTalkOptions{WebhookURL: "x", MsgType: "bogus"}); err == nil {
		t.Error("expected error for invalid msg_type")
	}
	if _, err := NewDingTalk(DingTalkOptions{WebhookURL: "x", MessageTemplate: "{{.Bogus"}); err == nil {
		t.Error("expected template parse error")
	}
}
