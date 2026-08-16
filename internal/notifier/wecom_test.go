package notifier

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWeComTextPayload(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"errcode":0,"errmsg":"ok"}`)

	w, err := NewWeCom(WeComOptions{WebhookURL: srv.URL + "?key=abc"})
	if err != nil {
		t.Fatalf("NewWeCom: %v", err)
	}
	if err := w.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		MsgType string `json:"msgtype"`
		Text    struct {
			Content       string   `json:"content"`
			MentionedList []string `json:"mentioned_list"`
		} `json:"text"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.MsgType != "text" {
		t.Errorf("msgtype = %q, want text", payload.MsgType)
	}
	if !strings.Contains(payload.Text.Content, "alice") {
		t.Errorf("content missing username: %q", payload.Text.Content)
	}
	if payload.Text.MentionedList != nil {
		t.Errorf("mentioned_list should be omitted when unset: %v", payload.Text.MentionedList)
	}
}

func TestWeComMentions(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"errcode":0}`)

	w, err := NewWeCom(WeComOptions{
		WebhookURL:          srv.URL,
		MentionedList:       []string{"@all"},
		MentionedMobileList: []string{"13800000000"},
	})
	if err != nil {
		t.Fatalf("NewWeCom: %v", err)
	}
	if err := w.Send(context.Background(), testEvent()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	var payload struct {
		Text struct {
			MentionedList       []string `json:"mentioned_list"`
			MentionedMobileList []string `json:"mentioned_mobile_list"`
		} `json:"text"`
	}
	if err := json.Unmarshal(*body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(payload.Text.MentionedList) != 1 || payload.Text.MentionedList[0] != "@all" {
		t.Errorf("mentioned_list = %v, want [@all]", payload.Text.MentionedList)
	}
	if len(payload.Text.MentionedMobileList) != 1 {
		t.Errorf("mentioned_mobile_list = %v", payload.Text.MentionedMobileList)
	}
}

func TestWeComMarkdownPayload(t *testing.T) {
	srv, body, _ := captureWebhook(t, `{"errcode":0}`)

	w, err := NewWeCom(WeComOptions{WebhookURL: srv.URL, MsgType: "markdown"})
	if err != nil {
		t.Fatalf("NewWeCom: %v", err)
	}
	if err := w.Send(context.Background(), testEvent()); err != nil {
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
	if !strings.Contains(payload.Markdown["content"], "**User:**") {
		t.Errorf("markdown body not used: %q", payload.Markdown["content"])
	}
}

func TestWeComAPIError(t *testing.T) {
	srv, _, _ := captureWebhook(t, `{"errcode":93000,"errmsg":"invalid webhook url"}`)

	w, err := NewWeCom(WeComOptions{WebhookURL: srv.URL})
	if err != nil {
		t.Fatalf("NewWeCom: %v", err)
	}
	err = w.Send(context.Background(), testEvent())
	if err == nil || !strings.Contains(err.Error(), "93000") {
		t.Errorf("expected api error, got %v", err)
	}
}

func TestWeComValidation(t *testing.T) {
	if _, err := NewWeCom(WeComOptions{}); err == nil {
		t.Error("expected error for empty webhook_url")
	}
	if _, err := NewWeCom(WeComOptions{WebhookURL: "x", MsgType: "bogus"}); err == nil {
		t.Error("expected error for invalid msg_type")
	}
	if _, err := NewWeCom(WeComOptions{
		WebhookURL:    "x",
		MsgType:       "markdown",
		MentionedList: []string{"@all"},
	}); err == nil {
		t.Error("expected error for mentions with markdown")
	}
	if _, err := NewWeCom(WeComOptions{WebhookURL: "x", MessageTemplate: "{{.Bogus"}); err == nil {
		t.Error("expected template parse error")
	}
}
