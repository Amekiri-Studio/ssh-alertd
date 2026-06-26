// Command ssh-alertd watches sshd logs and sends an alert on every successful
// SSH login. Backends are pluggable; Telegram is implemented today.
package main

import (
	"context"
	"flag"
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

	notifiers := buildNotifiers(cfg, logger)
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
func buildNotifiers(cfg *config.Config, logger *log.Logger) []notifier.Notifier {
	var ns []notifier.Notifier

	if cfg.Notifiers.Telegram.Enabled {
		tg := notifier.NewTelegram(
			cfg.Notifiers.Telegram.BotToken,
			cfg.Notifiers.Telegram.ChatID,
			cfg.Notifiers.Telegram.APIBase,
		)
		ns = append(ns, tg)
		logger.Printf("enabled notifier: telegram")
	}

	// Future backends (whatsapp, wecom, dingtalk, feishu, smtp) register here.

	return ns
}
