package notifier

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"ssh-alertd/internal/event"
)

// SMTP encryption modes.
const (
	smtpStartTLS = "starttls" // upgrade a plaintext connection (usually port 587)
	smtpImplicit = "tls"      // implicit TLS / SMTPS (usually port 465)
	smtpNone     = "none"     // no transport security (usually port 25)
)

// SMTP delivers alerts as email over SMTP.
type SMTP struct {
	host       string
	port       int
	username   string
	password   string
	from       string
	to         []string
	encryption string
}

// NewSMTP builds an SMTP notifier. encryption is one of "starttls", "tls" or
// "none"; when empty it is inferred from the port (465 → tls, else starttls).
func NewSMTP(host string, port int, username, password, from string, to []string, encryption string) *SMTP {
	if encryption == "" {
		if port == 465 {
			encryption = smtpImplicit
		} else {
			encryption = smtpStartTLS
		}
	}
	return &SMTP{
		host:       host,
		port:       port,
		username:   username,
		password:   password,
		from:       from,
		to:         append([]string(nil), to...),
		encryption: encryption,
	}
}

// Name implements Notifier.
func (s *SMTP) Name() string { return "smtp" }

// Send implements Notifier by delivering one email per event. The context
// bounds the initial connection and the overall exchange via a deadline.
func (s *SMTP) Send(ctx context.Context, e event.LoginEvent) error {
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}

	tlsConfig := &tls.Config{ServerName: s.host}
	if s.encryption == smtpImplicit {
		conn = tls.Client(conn, tlsConfig)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if s.encryption == smtpStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("server %s does not advertise STARTTLS", s.host)
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	for _, rcpt := range s.to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT TO %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := w.Write([]byte(s.message(e))); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close body: %w", err)
	}
	return client.Quit()
}

// message renders an RFC 5322 message with CRLF line endings. The subject is
// kept ASCII so no header encoding is required; the UTF-8 body carries the
// detail.
func (s *SMTP) message(e event.LoginEvent) string {
	subject := fmt.Sprintf("[ssh-alertd] SSH login: %s@%s from %s",
		e.Username, e.Hostname, e.IP)

	headers := []string{
		"Date: " + e.Time.Format(time.RFC1123Z),
		"From: " + s.from,
		"To: " + strings.Join(s.to, ", "),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}

	body := strings.ReplaceAll(plainBody(e), "\n", "\r\n")
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body + "\r\n"
}
