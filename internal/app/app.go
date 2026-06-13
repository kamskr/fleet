package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kamskr/fleet/internal/session"
	"github.com/kamskr/fleet/internal/store"
	"github.com/kamskr/fleet/internal/tmux"
)

type App struct {
	Store store.Store
	Tmux  *tmux.Client
}

func (a App) Sessions(ctx context.Context) ([]session.Session, error) {
	sessions, err := a.Store.Load()
	if err != nil {
		return nil, err
	}
	names, tmuxErr := a.Tmux.ListSessions(ctx)
	sessions = session.ReconcileStatus(sessions, names, tmuxErr)
	changed := false
	for i := range sessions {
		if sessions[i].Status != session.StatusRunning {
			sessions[i].LastResponse = "not running"
			continue
		}
		pane, err := a.Tmux.CapturePane(ctx, sessions[i].TmuxSessionName)
		if err != nil {
			sessions[i].LastResponse = "capture unavailable"
			continue
		}
		sessions[i].LastResponse = lastPaneLine(pane)
		hash := paneHash(pane)
		if sessions[i].LastPaneHash != hash {
			now := time.Now().UTC()
			sessions[i].LastPaneHash = hash
			sessions[i].LastActivityAt = &now
			changed = true
		}
	}
	if changed {
		_ = a.Store.Save(sessions)
	}
	session.Sort(sessions)
	return sessions, nil
}

func paneHash(pane string) string {
	sum := sha256.Sum256([]byte(pane))
	return hex.EncodeToString(sum[:])
}

func lastPaneLine(pane string) string {
	lines := strings.Split(pane, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return "waiting for output"
}

func (a App) Create(ctx context.Context, displayName, dir, command string) (session.Session, error) {
	if strings.TrimSpace(dir) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return session.Session{}, err
		}
		dir = cwd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return session.Session{}, err
	}
	if strings.TrimSpace(command) == "" {
		command = "bash"
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = filepath.Base(abs)
		if displayName == "." || displayName == string(filepath.Separator) || displayName == "" {
			displayName = "Fleet Session"
		}
	}
	s := session.New(displayName, abs, command)
	if err := a.Tmux.NewSession(ctx, s.TmuxSessionName, s.Directory, s.Command); err != nil {
		return session.Session{}, err
	}
	s.Status = session.StatusRunning
	return s, a.Store.Upsert(s)
}

func (a App) Resolve(ctx context.Context, q string) (session.Session, error) {
	sessions, err := a.Sessions(ctx)
	if err != nil {
		return session.Session{}, err
	}
	if s, ok := session.Resolve(sessions, q); ok {
		return s, nil
	}
	return session.Session{}, fmt.Errorf("session not found: %s", q)
}

func (a App) TogglePin(ctx context.Context, id string) error {
	s, err := a.Resolve(ctx, id)
	if err != nil {
		return err
	}
	s.Pinned = !s.Pinned
	return a.Store.Upsert(s)
}

func (a App) Rename(ctx context.Context, id, name string) error {
	s, err := a.Resolve(ctx, id)
	if err != nil {
		return err
	}
	s.DisplayName = strings.TrimSpace(name)
	if s.DisplayName == "" {
		return fmt.Errorf("display name cannot be empty")
	}
	return a.Store.Upsert(s)
}

func (a App) ChangeDir(ctx context.Context, id, dir string) error {
	s, err := a.Resolve(ctx, id)
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	s.Directory = abs
	return a.Store.Upsert(s)
}

func (a App) Kill(ctx context.Context, id string) error {
	s, err := a.Resolve(ctx, id)
	if err != nil {
		return err
	}
	if s.Status == session.StatusRunning {
		if err := a.Tmux.KillSession(ctx, s.TmuxSessionName); err != nil {
			return err
		}
	}
	return a.Store.Delete(s.ID)
}
