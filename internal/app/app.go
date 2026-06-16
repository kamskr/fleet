package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
		sessions[i].LastResponse = lastPanePreview(pane)
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

func lastPanePreview(pane string) string {
	lines := strings.Split(strings.ReplaceAll(stripANSI(pane), "\r\n", "\n"), "\n")
	for i := len(lines) - 1; i >= 0; {
		for i >= 0 && strings.TrimSpace(lines[i]) == "" {
			i--
		}
		if i < 0 {
			break
		}
		end := i + 1
		for i >= 0 && strings.TrimSpace(lines[i]) != "" {
			i--
		}
		preview := previewBlock(lines[i+1 : end])
		if preview != "" && !looksLikePrompt(preview) {
			return preview
		}
	}
	return "waiting for output"
}

func previewBlock(lines []string) string {
	words := []string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "│┃║┆┊| ")
		if line == "" {
			continue
		}
		words = append(words, strings.Fields(line)...)
	}
	if len(words) == 0 {
		return ""
	}
	if len(words) > 18 {
		words = append(words[:18], "…")
	}
	return strings.Join(words, " ")
}

func looksLikePrompt(s string) bool {
	if strings.Contains(s, " C-- back ") || strings.HasPrefix(s, "Fleet fleet-") {
		return true
	}
	fields := strings.Fields(s)
	if len(fields) <= 6 && (strings.HasPrefix(s, "~/") || strings.HasPrefix(s, "/") || strings.HasPrefix(s, "$ ") || strings.HasPrefix(s, "> ") || strings.HasPrefix(s, "❯ ")) {
		return true
	}
	return false
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
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
