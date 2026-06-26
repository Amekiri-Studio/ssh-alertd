package monitor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"

	"ssh-alertd/internal/config"
)

// Source produces a stream of raw log lines from sshd. Implementations must
// close the returned channel when ctx is cancelled or the underlying stream
// ends.
type Source interface {
	// Name identifies the source in logs.
	Name() string
	// Lines starts streaming and returns a channel of raw log lines.
	Lines(ctx context.Context) (<-chan string, error)
}

// NewSource builds the Source selected by the configuration.
func NewSource(cfg config.LogSourceConfig) (Source, error) {
	switch cfg.Type {
	case config.SourceJournald:
		// Follow only sshd-originated journal entries. Both the classic "sshd"
		// identifier and the newer privilege-separated "sshd-session" are
		// matched so we work across OpenSSH versions.
		return &commandSource{
			name: "journald",
			cmd:  "journalctl",
			args: []string{"-f", "-n", "0", "-o", "cat", "-t", "sshd", "-t", "sshd-session"},
		}, nil
	case config.SourceFile:
		// tail -F follows by name and transparently handles log rotation and
		// truncation.
		return &commandSource{
			name: "file:" + cfg.Path,
			cmd:  "tail",
			args: []string{"-F", "-n", "0", cfg.Path},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported log source type %q", cfg.Type)
	}
}

// commandSource streams stdout, one line per channel send, from a long-running
// follower command (journalctl -f or tail -F).
type commandSource struct {
	name string
	cmd  string
	args []string
}

func (s *commandSource) Name() string { return s.name }

func (s *commandSource) Lines(ctx context.Context) (<-chan string, error) {
	cmd := exec.CommandContext(ctx, s.cmd, s.args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", s.cmd, err)
	}

	out := make(chan string)
	go func() {
		defer close(out)
		scanner := bufio.NewScanner(stdout)
		// SSH log lines (esp. publickey fingerprints) can be long; grow the
		// scanner buffer well past bufio's default 64KiB limit.
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case out <- scanner.Text():
			case <-ctx.Done():
				_ = cmd.Wait()
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "ssh-alertd source %s scan error: %v\n", s.name, err)
		}
		_ = cmd.Wait()
	}()
	return out, nil
}
