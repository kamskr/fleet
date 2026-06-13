package session

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusIdle    Status = "idle"
	StatusDead    Status = "dead"
	StatusUnknown Status = "unknown"
)

type Session struct {
	ID              string     `json:"id"`
	DisplayName     string     `json:"display_name"`
	TmuxSessionName string     `json:"tmux_session_name"`
	Directory       string     `json:"directory"`
	Command         string     `json:"command"`
	Pinned          bool       `json:"pinned"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastActivityAt  *time.Time `json:"last_activity_at,omitempty"`
	Status          Status     `json:"status"`
	LastResponse    string     `json:"last_response,omitempty"`
	LastPaneHash    string     `json:"last_pane_hash,omitempty"`
}

func New(displayName, dir, command string) Session {
	now := time.Now().UTC()
	id := shortID()
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "Fleet Session"
	}
	return Session{
		ID:              id,
		DisplayName:     name,
		TmuxSessionName: TmuxName(name, id),
		Directory:       strings.TrimSpace(dir),
		Command:         strings.TrimSpace(command),
		CreatedAt:       now,
		UpdatedAt:       now,
		Status:          StatusUnknown,
	}
}

func TmuxName(displayName, id string) string {
	slug := slugify(displayName)
	if slug == "" {
		slug = "session"
	}
	return "fleet-" + slug + "-" + id
}

func Sort(sessions []Session) {
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].Pinned != sessions[j].Pinned {
			return sessions[i].Pinned
		}
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
}

func Resolve(sessions []Session, q string) (Session, bool) {
	q = strings.TrimSpace(q)
	for _, s := range sessions {
		if s.ID == q || s.DisplayName == q || s.TmuxSessionName == q {
			return s, true
		}
	}
	lower := strings.ToLower(q)
	for _, s := range sessions {
		if strings.ToLower(s.DisplayName) == lower {
			return s, true
		}
	}
	return Session{}, false
}

func ReconcileStatus(sessions []Session, tmuxNames []string, tmuxErr error) []Session {
	known := map[string]bool{}
	for _, n := range tmuxNames {
		known[n] = true
	}
	for i := range sessions {
		if tmuxErr != nil {
			sessions[i].Status = StatusUnknown
		} else if known[sessions[i].TmuxSessionName] {
			sessions[i].Status = StatusRunning
		} else {
			sessions[i].Status = StatusDead
		}
	}
	return sessions
}

func shortID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))[:8]
	}
	return hex.EncodeToString(b)
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlug.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 32 {
		s = strings.Trim(s[:32], "-")
	}
	return s
}
