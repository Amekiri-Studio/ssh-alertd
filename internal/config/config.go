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

// NotifiersConfig groups all notifier backends. Only Telegram is implemented
// today; the remaining fields are placeholders so the schema is stable as new
// backends land.
type NotifiersConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	// Reserved for future backends.
	WhatsApp map[string]any `json:"whatsapp,omitempty"`
	WeCom    map[string]any `json:"wecom,omitempty"`
	DingTalk map[string]any `json:"dingtalk,omitempty"`
	Feishu   map[string]any `json:"feishu,omitempty"`
	SMTP     map[string]any `json:"smtp,omitempty"`
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
	return nil
}
