package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kamskr/fleet/internal/app"
	"github.com/kamskr/fleet/internal/config"
	"github.com/kamskr/fleet/internal/store"
	"github.com/kamskr/fleet/internal/tmux"
	"github.com/kamskr/fleet/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fleet:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	tc, err := tmux.New(cfg.Socket)
	if err != nil {
		return err
	}
	a := app.App{Store: store.NewJSON(cfg.StatePath), Tmux: tc}
	ctx := context.Background()
	args := os.Args[1:]
	if len(args) == 0 {
		return tui.Run(a)
	}
	switch args[0] {
	case "new":
		s, err := a.Create(ctx, "", "", "")
		if err != nil {
			return err
		}
		fmt.Printf("created %s (%s)\n", s.DisplayName, s.ID)
	case "list":
		sessions, err := a.Sessions(ctx)
		if err != nil {
			return err
		}
		for _, s := range sessions {
			pin := " "
			if s.Pinned {
				pin = "*"
			}
			fmt.Printf("%s %-8s %-8s %-24s %s — %s\n", pin, s.ID, s.Status, s.DisplayName, s.Directory, commandLabel(s.Command))
		}
	case "attach":
		if len(args) < 2 {
			return fmt.Errorf("usage: fleet attach <id-or-name>")
		}
		s, err := a.Resolve(ctx, strings.Join(args[1:], " "))
		if err != nil {
			return err
		}
		return tc.Attach(ctx, s.TmuxSessionName)
	case "kill":
		if len(args) < 2 {
			return fmt.Errorf("usage: fleet kill <id-or-name>")
		}
		return a.Kill(ctx, strings.Join(args[1:], " "))
	case "doctor":
		return doctor(ctx, cfg, tc)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func commandLabel(command string) string {
	if strings.TrimSpace(command) == "" {
		return "default shell"
	}
	return command
}

func doctor(ctx context.Context, cfg config.Config, tc *tmux.Client) error {
	checks := tc.Doctor(ctx, cfg.StatePath)
	stateDir := filepath.Dir(cfg.StatePath)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		checks = append(checks, tmux.Check{Name: "state directory", OK: false, Info: err.Error()})
	} else {
		checks = append(checks, tmux.Check{Name: "state directory", OK: true, Info: stateDir})
	}
	f, err := os.OpenFile(cfg.StatePath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		checks = append(checks, tmux.Check{Name: "state file", OK: false, Info: err.Error()})
	} else {
		_ = f.Close()
		checks = append(checks, tmux.Check{Name: "state file", OK: true, Info: cfg.StatePath})
	}
	for _, c := range checks {
		mark := "ok"
		if !c.OK {
			mark = "fail"
		}
		fmt.Printf("%-4s %s — %s\n", mark, c.Name, c.Info)
	}
	return nil
}
