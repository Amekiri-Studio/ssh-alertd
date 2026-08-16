// Command ssh-alertd watches sshd logs and sends an alert on every successful
// SSH login. Backends are pluggable; Telegram and SMTP are implemented today.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ssh-alertd/internal/config"
	"ssh-alertd/internal/monitor"
	"ssh-alertd/internal/notifier"
)

func main() {
	configPath := flag.String("config", "/etc/ssh-alertd/config.json", "path to the JSON config file")
	flag.Parse()

	logger := log.New(os.Stderr, "ssh-alertd ", log.LstdFlags|log.Lmsgprefix)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("config: %v", err)
	}

	hostname := cfg.Hostname
	if hostname == "" {
		if h, err := os.Hostname(); err == nil {
			hostname = h
		} else {
			hostname = "unknown"
		}
	}

	notifiers, err := buildNotifiers(cfg, logger)
	if err != nil {
		logger.Fatalf("notifiers: %v", err)
	}
	if len(notifiers) == 0 {
		logger.Fatalf("no notifiers enabled; nothing to do")
	}
	dispatcher := notifier.NewDispatcher(logger, 10*time.Second, notifiers...)

	src, err := monitor.NewSource(cfg.LogSource)
	if err != nil {
		logger.Fatalf("log source: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mon := monitor.New(src, hostname, logger)
	if err := mon.Run(ctx, dispatcher.Dispatch); err != nil && err != context.Canceled {
		logger.Fatalf("monitor: %v", err)
	}
	logger.Printf("shutting down")
}

// buildNotifiers constructs the enabled notifier backends from config.
func buildNotifiers(cfg *config.Config, logger *log.Logger) ([]notifier.Notifier, error) {
	var ns []notifier.Notifier

	if cfg.Notifiers.Telegram.Enabled {
		t := cfg.Notifiers.Telegram

		// A message template file, when set, overrides the inline template.
		message := t.MessageTemplate
		if t.MessageTemplateFile != "" {
			data, err := os.ReadFile(t.MessageTemplateFile)
			if err != nil {
				return nil, fmt.Errorf("telegram message_template_file: %w", err)
			}
			message = string(data)
		}

		tg, err := notifier.NewTelegram(notifier.TelegramOptions{
			BotToken:        t.BotToken,
			ChatID:          t.ChatID,
			APIBase:         t.APIBase,
			MessageTemplate: message,
			ParseMode:       t.ParseMode,
		})
		if err != nil {
			return nil, fmt.Errorf("telegram: %w", err)
		}
		ns = append(ns, tg)
		logger.Printf("enabled notifier: telegram")
	}

	if cfg.Notifiers.SMTP.Enabled {
		s := cfg.Notifiers.SMTP

		// A body template file, when set, overrides the inline body template.
		body := s.BodyTemplate
		if s.BodyTemplateFile != "" {
			data, err := os.ReadFile(s.BodyTemplateFile)
			if err != nil {
				return nil, fmt.Errorf("smtp body_template_file: %w", err)
			}
			body = string(data)
		}

		sm, err := notifier.NewSMTP(notifier.SMTPOptions{
			Host:            s.Host,
			Port:            s.Port,
			Username:        s.Username,
			Password:        s.Password,
			From:            s.From,
			To:              s.To,
			Encryption:      s.Encryption,
			SubjectTemplate: s.SubjectTemplate,
			BodyTemplate:    body,
			HTML:            s.HTML,
		})
		if err != nil {
			return nil, fmt.Errorf("smtp: %w", err)
		}
		ns = append(ns, sm)
		logger.Printf("enabled notifier: smtp")
	}

	if cfg.Notifiers.Feishu.Enabled {
		f := cfg.Notifiers.Feishu

		message, err := resolveTemplate("feishu", f.MessageTemplate, f.MessageTemplateFile)
		if err != nil {
			return nil, err
		}

		fs, err := notifier.NewFeishu(notifier.FeishuOptions{
			WebhookURL:      f.WebhookURL,
			Secret:          f.Secret,
			MsgType:         f.MsgType,
			MessageTemplate: message,
		})
		if err != nil {
			return nil, fmt.Errorf("feishu: %w", err)
		}
		ns = append(ns, fs)
		logger.Printf("enabled notifier: feishu")
	}

	if cfg.Notifiers.DingTalk.Enabled {
		d := cfg.Notifiers.DingTalk

		message, err := resolveTemplate("dingtalk", d.MessageTemplate, d.MessageTemplateFile)
		if err != nil {
			return nil, err
		}

		dt, err := notifier.NewDingTalk(notifier.DingTalkOptions{
			WebhookURL:      d.WebhookURL,
			Secret:          d.Secret,
			MsgType:         d.MsgType,
			Title:           d.Title,
			MessageTemplate: message,
		})
		if err != nil {
			return nil, fmt.Errorf("dingtalk: %w", err)
		}
		ns = append(ns, dt)
		logger.Printf("enabled notifier: dingtalk")
	}

	if cfg.Notifiers.WeCom.Enabled {
		w := cfg.Notifiers.WeCom

		message, err := resolveTemplate("wecom", w.MessageTemplate, w.MessageTemplateFile)
		if err != nil {
			return nil, err
		}

		wc, err := notifier.NewWeCom(notifier.WeComOptions{
			WebhookURL:          w.WebhookURL,
			MsgType:             w.MsgType,
			MessageTemplate:     message,
			MentionedList:       w.MentionedList,
			MentionedMobileList: w.MentionedMobileList,
		})
		if err != nil {
			return nil, fmt.Errorf("wecom: %w", err)
		}
		ns = append(ns, wc)
		logger.Printf("enabled notifier: wecom")
	}

	// Future backends (whatsapp) register here.

	return ns, nil
}

// resolveTemplate returns the template text to use for a backend: the contents
// of file when set, otherwise the inline template. backend names the notifier in
// error messages.
func resolveTemplate(backend, inline, file string) (string, error) {
	if file == "" {
		return inline, nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("%s message_template_file: %w", backend, err)
	}
	return string(data), nil
}
