package notifier

import (
	"strings"
	"testing"
	"time"

	"ssh-alertd/internal/event"
)

func testEvent() event.LoginEvent {
	return event.LoginEvent{
		Username: "alice",
		IP:       "203.0.113.5",
		Port:     "50568",
		Method:   "publickey",
		Hostname: "myhost",
		Time:     time.Date(2026, 6, 26, 11, 0, 0, 0, time.UTC),
	}
}

func mustSMTP(t *testing.T, o SMTPOptions) *SMTP {
	t.Helper()
	if o.Host == "" {
		o.Host = "smtp.example.com"
	}
	if o.From == "" {
		o.From = "a@b.c"
	}
	if len(o.To) == 0 {
		o.To = []string{"x@y.z"}
	}
	s, err := NewSMTP(o)
	if err != nil {
		t.Fatalf("NewSMTP: %v", err)
	}
	return s
}

func TestNewSMTPInfersEncryption(t *testing.T) {
	cases := []struct {
		port int
		enc  string
		want string
	}{
		{587, "", smtpStartTLS},
		{465, "", smtpImplicit},
		{25, "", smtpStartTLS},
		{587, "none", smtpNone},
		{465, "starttls", smtpStartTLS},
	}
	for _, tc := range cases {
		s := mustSMTP(t, SMTPOptions{Port: tc.port, Encryption: tc.enc})
		if s.encryption != tc.want {
			t.Errorf("port=%d enc=%q -> %q, want %q", tc.port, tc.enc, s.encryption, tc.want)
		}
	}
}

func TestSMTPMessageDefault(t *testing.T) {
	s := mustSMTP(t, SMTPOptions{
		Port: 587, Username: "u", Password: "p", From: "alert@example.com",
		To: []string{"a@example.com", "b@example.com"}, Encryption: "starttls",
	})
	msg, err := s.message(testEvent())
	if err != nil {
		t.Fatalf("message: %v", err)
	}

	for _, want := range []string{
		"From: alert@example.com\r\n",
		"To: a@example.com, b@example.com\r\n",
		"Subject: [ssh-alertd] SSH login: alice@myhost from 203.0.113.5\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"alice",
		"203.0.113.5",
		"50568",
		"\r\n\r\n", // header/body separator
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q\n--- message ---\n%s", want, msg)
		}
	}
}

func TestSMTPCustomTemplates(t *testing.T) {
	s := mustSMTP(t, SMTPOptions{
		Port:            25,
		Encryption:      "none",
		SubjectTemplate: "Login {{.Username}} on {{.Hostname}}",
		BodyTemplate:    "User={{.Username}} IP={{.IP}} Port={{.Port}} Method={{.Method}}",
	})
	msg, err := s.message(testEvent())
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	if !strings.Contains(msg, "Subject: Login alice on myhost\r\n") {
		t.Errorf("custom subject not rendered\n%s", msg)
	}
	if !strings.Contains(msg, "User=alice IP=203.0.113.5 Port=50568 Method=publickey") {
		t.Errorf("custom body not rendered\n%s", msg)
	}
}

func TestSMTPHTMLTemplate(t *testing.T) {
	s := mustSMTP(t, SMTPOptions{
		Port:         587,
		Encryption:   "starttls",
		HTML:         true,
		BodyTemplate: "<b>{{.Username}}</b> from {{.IP}}",
	})
	msg, err := s.message(testEvent())
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	if !strings.Contains(msg, "Content-Type: text/html; charset=UTF-8\r\n") {
		t.Errorf("expected html content type\n%s", msg)
	}
	if !strings.Contains(msg, "<b>alice</b> from 203.0.113.5") {
		t.Errorf("html body not rendered\n%s", msg)
	}
}

func TestSMTPNonASCIISubjectIsEncoded(t *testing.T) {
	s := mustSMTP(t, SMTPOptions{
		Port:            587,
		Encryption:      "starttls",
		SubjectTemplate: "登录告警 {{.Username}}",
	})
	msg, err := s.message(testEvent())
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	// Non-ASCII subjects must be MIME word-encoded, not raw UTF-8 in the header.
	if !strings.Contains(msg, "Subject: =?utf-8?") {
		t.Errorf("non-ASCII subject not MIME-encoded\n%s", msg)
	}
	if strings.Contains(msg, "Subject: 登录告警") {
		t.Errorf("raw non-ASCII leaked into Subject header\n%s", msg)
	}
}

func TestNewSMTPRejectsBadTemplate(t *testing.T) {
	if _, err := NewSMTP(SMTPOptions{
		Host: "h", Port: 25, From: "f@x", To: []string{"t@x"}, Encryption: "none",
		SubjectTemplate: "{{.Username", // unclosed action
	}); err == nil {
		t.Fatal("expected error for malformed subject template, got nil")
	}
}
