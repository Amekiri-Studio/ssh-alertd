// Package config loads and validates the daemon configuration from a JSON file.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Config is the root configuration object.
type Config struct {
	// Hostname overrides the host name reported in alerts. Empty means the
	// daemon uses the OS hostname.
	Hostname string `json:"hostname"`

	// LogSource controls where SSH log lines are read from.
	LogSource LogSourceConfig `json:"log_source"`

	// Notifiers configures each supported notification backend.
	Notifiers NotifiersConfig `json:"notifiers"`
}

// SourceType enumerates the supported log sources.
type SourceType string

const (
	// SourceJournald reads sshd messages from the systemd journal.
	SourceJournald SourceType = "journald"
	// SourceFile tails a classic auth log file (e.g. /var/log/auth.log).
	SourceFile SourceType = "file"
)

// LogSourceConfig selects and parameterizes the log source.
type LogSourceConfig struct {
	Type SourceType `json:"type"`
	// Path is the log file to tail when Type is "file".
	Path string `json:"path"`
}

// NotifiersConfig groups all notifier backends. Telegram and SMTP are
// implemented today; the remaining fields are placeholders so the schema is
// stable as new backends land.
type NotifiersConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	SMTP     SMTPConfig     `json:"smtp"`
	// Reserved for future backends.
	WhatsApp map[string]any `json:"whatsapp,omitempty"`
	WeCom    map[string]any `json:"wecom,omitempty"`
	DingTalk map[string]any `json:"dingtalk,omitempty"`
	Feishu   map[string]any `json:"feishu,omitempty"`
}

// TelegramConfig holds the Telegram Bot API credentials.
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	// APIBase optionally overrides the Telegram API endpoint, useful behind a
	// reverse proxy. Defaults to https://api.telegram.org when empty.
	APIBase string `json:"api_base"`
}

// SMTPConfig holds the settings for sending alerts over SMTP (email).
type SMTPConfig struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	// From is the envelope/header sender address.
	From string `json:"from"`
	// To is the list of recipient addresses (at least one).
	To []string `json:"to"`
	// Encryption selects the transport security: "starttls" (default, typically
	// port 587), "tls" (implicit TLS / SMTPS, typically port 465) or "none".
	Encryption string `json:"encryption"`
}

// Load reads, parses and validates the config file at path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	cfg.applyDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.LogSource.Type == "" {
		c.LogSource.Type = SourceJournald
	}
	if c.LogSource.Type == SourceFile && c.LogSource.Path == "" {
		c.LogSource.Path = "/var/log/auth.log"
	}
	if c.Notifiers.Telegram.APIBase == "" {
		c.Notifiers.Telegram.APIBase = "https://api.telegram.org"
	}

	smtp := &c.Notifiers.SMTP
	if smtp.Encryption == "" {
		if smtp.Port == 465 {
			smtp.Encryption = "tls"
		} else {
			smtp.Encryption = "starttls"
		}
	}
	if smtp.Port == 0 {
		if smtp.Encryption == "tls" {
			smtp.Port = 465
		} else {
			smtp.Port = 587
		}
	}
}

func (c *Config) validate() error {
	switch c.LogSource.Type {
	case SourceJournald, SourceFile:
	default:
		return fmt.Errorf("invalid log_source.type %q (want %q or %q)",
			c.LogSource.Type, SourceJournald, SourceFile)
	}

	if c.Notifiers.Telegram.Enabled {
		if c.Notifiers.Telegram.BotToken == "" {
			return fmt.Errorf("telegram enabled but bot_token is empty")
		}
		if c.Notifiers.Telegram.ChatID == "" {
			return fmt.Errorf("telegram enabled but chat_id is empty")
		}
	}

	if c.Notifiers.SMTP.Enabled {
		s := c.Notifiers.SMTP
		switch s.Encryption {
		case "starttls", "tls", "none":
		default:
			return fmt.Errorf("smtp enabled but encryption %q is invalid (want starttls, tls or none)", s.Encryption)
		}
		if s.Host == "" {
			return fmt.Errorf("smtp enabled but host is empty")
		}
		if s.From == "" {
			return fmt.Errorf("smtp enabled but from is empty")
		}
		if len(s.To) == 0 {
			return fmt.Errorf("smtp enabled but to is empty")
		}
	}
	return nil
}
