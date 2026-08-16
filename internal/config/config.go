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

// NotifiersConfig groups all notifier backends. Telegram, SMTP and the three
// Chinese group-robot webhooks are implemented today; WhatsApp remains a
// placeholder so the schema stays stable as new backends land.
type NotifiersConfig struct {
	Telegram TelegramConfig `json:"telegram"`
	SMTP     SMTPConfig     `json:"smtp"`
	Feishu   FeishuConfig   `json:"feishu"`
	DingTalk DingTalkConfig `json:"dingtalk"`
	WeCom    WeComConfig    `json:"wecom"`
	// Reserved for future backends.
	WhatsApp map[string]any `json:"whatsapp,omitempty"`
}

// TelegramConfig holds the Telegram Bot API credentials.
type TelegramConfig struct {
	Enabled  bool   `json:"enabled"`
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
	// APIBase optionally overrides the Telegram API endpoint, useful behind a
	// reverse proxy. Defaults to https://api.telegram.org when empty.
	APIBase string `json:"api_base"`
	// MessageTemplate is an optional Go template for the message text (fields
	// .Username .IP .Port .Method .Hostname .Time). Empty uses the built-in
	// HTML format.
	MessageTemplate string `json:"message_template"`
	// MessageTemplateFile, when set, is read as the message template and takes
	// precedence over MessageTemplate — convenient for multi-line messages.
	MessageTemplateFile string `json:"message_template_file"`
	// ParseMode is "HTML" (default), "MarkdownV2", "Markdown" or "none"; it
	// applies to a custom MessageTemplate. The built-in format is always HTML.
	ParseMode string `json:"parse_mode"`
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

	// SubjectTemplate is an optional Go text/template for the email subject.
	// Templates receive the login event with fields .Username .IP .Port .Method
	// .Hostname .Time (e.g. {{.Username}}, {{.Time.Format "2006-01-02 15:04:05"}}).
	// Empty uses the built-in subject.
	SubjectTemplate string `json:"subject_template"`
	// BodyTemplate is an optional Go template for the email body (same fields as
	// SubjectTemplate). Empty uses the built-in body.
	BodyTemplate string `json:"body_template"`
	// BodyTemplateFile, when set, is read as the body template and takes
	// precedence over BodyTemplate — convenient for multi-line bodies.
	BodyTemplateFile string `json:"body_template_file"`
	// HTML renders the body as text/html (using html/template for auto-escaping)
	// instead of text/plain. Pair it with a BodyTemplate.
	HTML bool `json:"html"`
}

// FeishuConfig holds the settings for a Feishu (飞书) / Lark custom group robot.
type FeishuConfig struct {
	Enabled bool `json:"enabled"`
	// WebhookURL is the full custom-bot URL shown when the robot is added to a
	// group (open.feishu.cn for Feishu, open.larksuite.com for Lark).
	WebhookURL string `json:"webhook_url"`
	// Secret enables signed requests ("签名校验"). Empty means the robot relies on
	// keyword or IP allowlist security instead.
	Secret string `json:"secret"`
	// MsgType is "text" (default) or "interactive" (message card).
	MsgType string `json:"msg_type"`
	// MessageTemplate is an optional Go template for the message (fields
	// .Username .IP .Port .Method .Hostname .Time). For "interactive" it must
	// render a JSON card object. Empty uses the built-in plain-text format.
	MessageTemplate string `json:"message_template"`
	// MessageTemplateFile, when set, is read as the message template and takes
	// precedence over MessageTemplate.
	MessageTemplateFile string `json:"message_template_file"`
}

// DingTalkConfig holds the settings for a DingTalk (钉钉) custom group robot.
type DingTalkConfig struct {
	Enabled bool `json:"enabled"`
	// WebhookURL is the full robot URL including the access_token parameter.
	WebhookURL string `json:"webhook_url"`
	// Secret enables signed requests ("加签"). DingTalk requires at least one
	// security setting: signing, a keyword, or an IP allowlist.
	Secret string `json:"secret"`
	// MsgType is "text" (default) or "markdown".
	MsgType string `json:"msg_type"`
	// Title is the markdown notification title shown in the conversation list
	// (markdown only). Empty uses a built-in title.
	Title string `json:"title"`
	// MessageTemplate is an optional Go template for the message body. Empty
	// uses the built-in plain-text or Markdown format.
	MessageTemplate string `json:"message_template"`
	// MessageTemplateFile, when set, is read as the message template and takes
	// precedence over MessageTemplate.
	MessageTemplateFile string `json:"message_template_file"`
}

// WeComConfig holds the settings for a WeCom (企业微信) group robot.
type WeComConfig struct {
	Enabled bool `json:"enabled"`
	// WebhookURL is the full robot URL including the key parameter. WeCom
	// robots have no signature scheme, so treat the key as a secret.
	WebhookURL string `json:"webhook_url"`
	// MsgType is "text" (default) or "markdown".
	MsgType string `json:"msg_type"`
	// MessageTemplate is an optional Go template for the message body. Empty
	// uses the built-in plain-text or Markdown format.
	MessageTemplate string `json:"message_template"`
	// MessageTemplateFile, when set, is read as the message template and takes
	// precedence over MessageTemplate.
	MessageTemplateFile string `json:"message_template_file"`
	// MentionedList are user IDs to @ in the group ("@all" mentions everyone).
	// Text messages only.
	MentionedList []string `json:"mentioned_list,omitempty"`
	// MentionedMobileList are phone numbers to @ in the group. Text only.
	MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
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
	if c.Notifiers.Telegram.ParseMode == "" {
		c.Notifiers.Telegram.ParseMode = "HTML"
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

	if c.Notifiers.Feishu.MsgType == "" {
		c.Notifiers.Feishu.MsgType = "text"
	}
	if c.Notifiers.DingTalk.MsgType == "" {
		c.Notifiers.DingTalk.MsgType = "text"
	}
	if c.Notifiers.WeCom.MsgType == "" {
		c.Notifiers.WeCom.MsgType = "text"
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
		switch c.Notifiers.Telegram.ParseMode {
		case "HTML", "MarkdownV2", "Markdown", "none":
		default:
			return fmt.Errorf("telegram parse_mode %q is invalid (want HTML, MarkdownV2, Markdown or none)",
				c.Notifiers.Telegram.ParseMode)
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

	if f := c.Notifiers.Feishu; f.Enabled {
		if f.WebhookURL == "" {
			return fmt.Errorf("feishu enabled but webhook_url is empty")
		}
		switch f.MsgType {
		case "text", "interactive":
		default:
			return fmt.Errorf("feishu msg_type %q is invalid (want text or interactive)", f.MsgType)
		}
		if f.MsgType == "interactive" && f.MessageTemplate == "" && f.MessageTemplateFile == "" {
			return fmt.Errorf(`feishu msg_type "interactive" requires message_template or message_template_file (a JSON card)`)
		}
	}

	if d := c.Notifiers.DingTalk; d.Enabled {
		if d.WebhookURL == "" {
			return fmt.Errorf("dingtalk enabled but webhook_url is empty")
		}
		switch d.MsgType {
		case "text", "markdown":
		default:
			return fmt.Errorf("dingtalk msg_type %q is invalid (want text or markdown)", d.MsgType)
		}
	}

	if w := c.Notifiers.WeCom; w.Enabled {
		if w.WebhookURL == "" {
			return fmt.Errorf("wecom enabled but webhook_url is empty")
		}
		switch w.MsgType {
		case "text", "markdown":
		default:
			return fmt.Errorf("wecom msg_type %q is invalid (want text or markdown)", w.MsgType)
		}
		if w.MsgType != "text" && (len(w.MentionedList) > 0 || len(w.MentionedMobileList) > 0) {
			return fmt.Errorf(`wecom mentioned_list/mentioned_mobile_list require msg_type "text"`)
		}
	}
	return nil
}
