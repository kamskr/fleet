package tui

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kamskr/fleet/internal/app"
	"github.com/kamskr/fleet/internal/session"
	"github.com/kamskr/fleet/internal/tmux"
)

type fakeStore struct{ sessions []session.Session }

func (f *fakeStore) Load() ([]session.Session, error) { return f.sessions, nil }
func (f *fakeStore) Save(s []session.Session) error {
	f.sessions = s
	return nil
}
func (f *fakeStore) Upsert(s session.Session) error {
	f.sessions = append(f.sessions, s)
	return nil
}
func (f *fakeStore) Delete(id string) error { return nil }

type fakeTmuxRunner struct {
	calls []struct {
		name string
		args []string
	}
}

func (f *fakeTmuxRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, struct {
		name string
		args []string
	}{name, append([]string{}, args...)})
	return nil, nil
}
func (f *fakeTmuxRunner) Attach(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, struct {
		name string
		args []string
	}{name, append([]string{}, args...)})
	return nil
}

func TestNewKeyCreatesDefaultShellSessionAndAttaches(t *testing.T) {
	store := &fakeStore{}
	runner := &fakeTmuxRunner{}
	m := model{app: app.App{Store: store, Tmux: &tmux.Client{Socket: "fleet-test", Runner: runner}}}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	updated := next.(model)
	if updated.mode != modeNormal {
		t.Fatalf("n should not open an input prompt, mode=%v", updated.mode)
	}
	if cmd == nil {
		t.Fatal("expected create-and-attach command")
	}
	msg := cmd()
	attach, ok := msg.(attachReadyMsg)
	if !ok {
		t.Fatalf("cmd returned %T, want attachReadyMsg", msg)
	}
	if attach == "" {
		t.Fatal("expected tmux session name to attach")
	}
	if len(store.sessions) != 1 {
		t.Fatalf("sessions=%d, want 1", len(store.sessions))
	}
	if store.sessions[0].Command != "" {
		t.Fatalf("command=%q, want empty default shell", store.sessions[0].Command)
	}
}

func TestRenderGroupedListKeepsRowsLeftAligned(t *testing.T) {
	m := model{grouped: true, sessions: []session.Session{
		{DisplayName: "Fleet", Directory: "/Users/kamskr/Files/Fleet", Status: session.StatusDead},
		{DisplayName: "frontdesk", Directory: "/Users/kamskr/Files/frontdesk", Status: session.StatusRunning, LastResponse: "~/Files/frontdesk:main  /status"},
	}}

	out := stripANSI(m.renderList(m.visible()))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	if !strings.HasPrefix(lines[2], "▸  Fleet") {
		t.Fatalf("first grouped row should start at the left edge, got %q", lines[2])
	}
	if !strings.HasPrefix(lines[7], "   frontdesk") {
		t.Fatalf("second grouped row should start at the left edge, got %q", lines[7])
	}
	if strings.Contains(lines[7], "/Users/kamskr/Files/frontdesk") {
		t.Fatalf("grouped row should not repeat directory column: %q", lines[7])
	}
}

func TestRenderGroupedListAddsSpaceBetweenGroups(t *testing.T) {
	m := model{grouped: true, sessions: []session.Session{
		{DisplayName: "a", Directory: "/a", Status: session.StatusDead},
		{DisplayName: "b", Directory: "/b", Status: session.StatusDead},
	}}

	out := stripANSI(m.renderList(m.visible()))
	if !strings.Contains(out, "▾ /a\n\n▸  a") {
		t.Fatalf("expected blank line between group header and row, got:\n%s", out)
	}
	if !strings.Contains(out, "▸  a    stopped\n\n\n▾ /b") {
		t.Fatalf("expected extra space between groups, got:\n%s", out)
	}
}

func TestRenderListDoesNotShowDirectoryColumn(t *testing.T) {
	m := model{sessions: []session.Session{
		{DisplayName: "frontdesk", Directory: "/Users/kamskr/Files/frontdesk", Status: session.StatusRunning, LastResponse: "~/Files/frontdesk:main  /status"},
	}}

	out := stripANSI(m.renderList(m.visible()))
	if strings.Contains(out, "/Users/kamskr/Files/frontdesk") {
		t.Fatalf("normal row should not show directory column: %q", out)
	}
	if !strings.Contains(out, "frontdesk") || !strings.Contains(out, "waiting for output") {
		t.Fatalf("normal row should still show identity and activity: %q", out)
	}
}

func TestResponsePreviewShowsSessionState(t *testing.T) {
	now := time.Now()
	idle := now.Add(-2 * time.Minute)
	working := now.Add(-3 * time.Second)

	tests := []struct {
		name string
		s    session.Session
		want string
	}{
		{
			name: "working",
			s:    session.Session{Status: session.StatusRunning, LastActivityAt: &working, LastResponse: "editing renderer tests"},
			want: "working: editing renderer tests",
		},
		{
			name: "idle",
			s:    session.Session{Status: session.StatusRunning, LastActivityAt: &idle, LastResponse: "waiting for user confirmation"},
			want: "idle 2m: waiting for user confirmation",
		},
		{
			name: "waiting",
			s:    session.Session{Status: session.StatusRunning},
			want: "waiting for output",
		},
		{
			name: "stopped",
			s:    session.Session{Status: session.StatusDead, LastResponse: "old response"},
			want: "stopped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := responsePreview(tt.s); got != tt.want {
				t.Fatalf("responsePreview() = %q, want %q", got, tt.want)
			}
		})
	}
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}
