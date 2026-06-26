package notifier

import (
	"strings"
	"testing"
	"time"

	"ssh-alertd/internal/event"
)

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
		s := NewSMTP("smtp.example.com", tc.port, "", "", "a@b.c", []string{"x@y.z"}, tc.enc)
		if s.encryption != tc.want {
			t.Errorf("port=%d enc=%q -> %q, want %q", tc.port, tc.enc, s.encryption, tc.want)
		}
	}
}

func TestSMTPMessage(t *testing.T) {
	s := NewSMTP("smtp.example.com", 587, "u", "p", "alert@example.com",
		[]string{"a@example.com", "b@example.com"}, "starttls")
	e := event.LoginEvent{
		Username: "alice",
		IP:       "203.0.113.5",
		Port:     "50568",
		Method:   "publickey",
		Hostname: "myhost",
		Time:     time.Date(2026, 6, 26, 11, 0, 0, 0, time.UTC),
	}
	msg := s.message(e)

	// Headers and key fields must be present, with CRLF line endings.
	for _, want := range []string{
		"\r\n",
		"From: alert@example.com\r\n",
		"To: a@example.com, b@example.com\r\n",
		"Subject: [ssh-alertd] SSH login: alice@myhost from 203.0.113.5\r\n",
		"Content-Type: text/plain; charset=UTF-8\r\n",
		"alice",
		"203.0.113.5",
		"50568",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q\n--- message ---\n%s", want, msg)
		}
	}

	// Header/body separator (blank line) must exist.
	if !strings.Contains(msg, "\r\n\r\n") {
		t.Error("message missing header/body separator")
	}
}
