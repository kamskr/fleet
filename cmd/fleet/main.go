package main

import (
	"bufio"
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
		name, dir, cmd, err := promptNew()
		if err != nil {
			return err
		}
		s, err := a.Create(ctx, name, dir, cmd)
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
			fmt.Printf("%s %-8s %-8s %-24s %s — %s\n", pin, s.ID, s.Status, s.DisplayName, s.Directory, s.Command)
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

func promptNew() (string, string, string, error) {
	r := bufio.NewReader(os.Stdin)
	read := func(label, def string) (string, error) {
		if def != "" {
			fmt.Printf("%s [%s]: ", label, def)
		} else {
			fmt.Printf("%s: ", label)
		}
		v, err := r.ReadString('\n')
		if err != nil {
			return "", err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			v = def
		}
		return v, nil
	}
	cwd, _ := os.Getwd()
	dir, err := read("Directory", cwd)
	if err != nil {
		return "", "", "", err
	}
	cmd, err := read("Command (opencode/claude/bash/custom)", "opencode")
	if err != nil {
		return "", "", "", err
	}
	return filepath.Base(dir), dir, cmd, nil
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
