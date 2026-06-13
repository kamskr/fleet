package tmux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Runner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
	Attach(ctx context.Context, name string, args ...string) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

func (ExecRunner) Attach(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type Client struct {
	Socket string
	Runner Runner
}

func New(socket string) (*Client, error) {
	if strings.TrimSpace(socket) == "" {
		return nil, errors.New("tmux socket cannot be empty")
	}
	return &Client{Socket: socket, Runner: ExecRunner{}}, nil
}

func (c *Client) args(args ...string) []string {
	out := []string{"-L", c.Socket}
	out = append(out, args...)
	return out
}

func (c *Client) NewSession(ctx context.Context, name, dir, command string) error {
	if !strings.HasPrefix(name, "fleet-") {
		return fmt.Errorf("tmux session name %q must start with fleet-", name)
	}
	args := c.args("new-session", "-d", "-s", name, "-c", dir, command)
	if out, err := c.Runner.Run(ctx, "tmux", args...); err != nil {
		return fmt.Errorf("tmux new-session failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if err := c.SetupFleetChrome(ctx, name); err != nil {
		return err
	}
	return nil
}

func (c *Client) SetupFleetChrome(ctx context.Context, name string) error {
	// Clean up the early MVP raw '-' binding. Ctrl-minus is reported to tmux as C-_
	// by common terminals, and C-g remains a reliable fallback.
	_, _ = c.Runner.Run(ctx, "tmux", c.args("unbind-key", "-n", "-")...)
	commands := [][]string{
		c.args("set-option", "-t", name, "status", "on"),
		c.args("set-option", "-t", name, "status-style", "bg=colour235,fg=colour250"),
		c.args("set-option", "-t", name, "status-left-style", "bg=colour39,fg=colour235,bold"),
		c.args("set-option", "-t", name, "status-left", " Fleet #{session_name} "),
		c.args("set-option", "-t", name, "status-right", " C-- back • C-g back "),
		c.args("set-option", "-t", name, "window-status-current-style", "fg=colour212,bold"),
		c.args("bind-key", "-n", "C-_", "detach-client"),
		c.args("bind-key", "-n", "C-g", "detach-client"),
	}
	for _, args := range commands {
		if out, err := c.Runner.Run(ctx, "tmux", args...); err != nil {
			return fmt.Errorf("tmux fleet chrome setup failed: %w: %s", err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func (c *Client) Attach(ctx context.Context, name string) error {
	if err := c.SetupFleetChrome(ctx, name); err != nil {
		return err
	}
	return c.Runner.Attach(ctx, "tmux", c.args("attach", "-t", name)...)
}

func (c *Client) AttachCommand(name string) *exec.Cmd {
	return exec.Command("tmux", c.args("attach", "-t", name)...)
}

func (c *Client) ListSessions(ctx context.Context) ([]string, error) {
	out, err := c.Runner.Run(ctx, "tmux", c.args("list-sessions", "-F", "#{session_name}")...)
	text := strings.TrimSpace(string(out))
	if err != nil {
		if strings.Contains(text, "no server running") || strings.Contains(text, "failed to connect") || strings.Contains(text, "error connecting") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("tmux list-sessions failed: %w: %s", err, text)
	}
	if text == "" {
		return []string{}, nil
	}
	return strings.Split(text, "\n"), nil
}

func (c *Client) KillSession(ctx context.Context, name string) error {
	out, err := c.Runner.Run(ctx, "tmux", c.args("kill-session", "-t", name)...)
	if err != nil {
		return fmt.Errorf("tmux kill-session failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *Client) CapturePane(ctx context.Context, name string) (string, error) {
	out, err := c.Runner.Run(ctx, "tmux", c.args("capture-pane", "-t", name, "-p")...)
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

type Check struct {
	Name string
	OK   bool
	Info string
}

func (c *Client) Doctor(ctx context.Context, statePath string) []Check {
	checks := []Check{}
	if _, err := exec.LookPath("tmux"); err != nil {
		checks = append(checks, Check{"tmux installed", false, err.Error()})
	} else {
		checks = append(checks, Check{"tmux installed", true, "found"})
	}
	if _, err := c.ListSessions(ctx); err != nil {
		checks = append(checks, Check{"tmux socket usable", false, err.Error()})
	} else {
		checks = append(checks, Check{"tmux socket usable", true, c.Socket})
	}
	return checks
}
