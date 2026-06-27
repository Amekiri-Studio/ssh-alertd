package notifier

import (
	"strings"
	"testing"

	"ssh-alertd/internal/event"
)

func TestTelegramDefaultMessage(t *testing.T) {
	tg, err := NewTelegram(TelegramOptions{BotToken: "x", ChatID: "1"})
	if err != nil {
		t.Fatalf("NewTelegram: %v", err)
	}
	if tg.parseMode != "HTML" {
		t.Errorf("default parseMode = %q, want HTML", tg.parseMode)
	}
	msg, err := tg.render(testEvent())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"<b>SSH Login Alert</b>", "<code>alice</code>", "<code>203.0.113.5</code>"} {
		if !strings.Contains(msg, want) {
			t.Errorf("default message missing %q\n%s", want, msg)
		}
	}
}

func TestTelegramCustomTemplateHTML(t *testing.T) {
	tg, err := NewTelegram(TelegramOptions{
		BotToken:        "x",
		ChatID:          "1",
		MessageTemplate: "<b>{{.Username}}</b> from {{.IP}}",
	})
	if err != nil {
		t.Fatalf("NewTelegram: %v", err)
	}
	if tg.parseMode != "HTML" {
		t.Errorf("parseMode = %q, want HTML", tg.parseMode)
	}
	msg, err := tg.render(testEvent())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if msg != "<b>alice</b> from 203.0.113.5" {
		t.Errorf("rendered = %q", msg)
	}
}

func TestTelegramHTMLTemplateEscapesFields(t *testing.T) {
	// A username with markup must not break the HTML; html/template escapes it.
	tg, err := NewTelegram(TelegramOptions{
		BotToken:        "x",
		ChatID:          "1",
		MessageTemplate: "<b>{{.Username}}</b>",
	})
	if err != nil {
		t.Fatalf("NewTelegram: %v", err)
	}
	e := testEvent()
	e.Username = "<script>"
	msg, err := tg.render(e)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if strings.Contains(msg, "<script>") {
		t.Errorf("field not escaped: %q", msg)
	}
	if !strings.Contains(msg, "&lt;script&gt;") {
		t.Errorf("expected escaped field, got %q", msg)
	}
}

func TestTelegramPlainParseModeNone(t *testing.T) {
	tg, err := NewTelegram(TelegramOptions{
		BotToken:        "x",
		ChatID:          "1",
		MessageTemplate: "{{.Username}} logged in",
		ParseMode:       "none",
	})
	if err != nil {
		t.Fatalf("NewTelegram: %v", err)
	}
	if tg.parseMode != "" {
		t.Errorf("parseMode for none = %q, want empty (plain)", tg.parseMode)
	}
	msg, _ := tg.render(event.LoginEvent{Username: "bob"})
	if msg != "bob logged in" {
		t.Errorf("rendered = %q", msg)
	}
}

func TestTelegramRejectsBadTemplate(t *testing.T) {
	if _, err := NewTelegram(TelegramOptions{
		BotToken: "x", ChatID: "1", MessageTemplate: "{{.Username",
	}); err == nil {
		t.Fatal("expected error for malformed template, got nil")
	}
}
