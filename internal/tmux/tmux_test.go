package tmux

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

type call struct {
	name string
	args []string
}
type fakeRunner struct {
	calls []call
	out   []byte
	err   error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, call{name, append([]string{}, args...)})
	return f.out, f.err
}
func (f *fakeRunner) Attach(_ context.Context, name string, args ...string) error {
	f.calls = append(f.calls, call{name, append([]string{}, args...)})
	return f.err
}

func TestCommandConstructionUsesFleetSocket(t *testing.T) {
	f := &fakeRunner{}
	c := &Client{Socket: "fleet", Runner: f}
	ctx := context.Background()
	_ = c.NewSession(ctx, "fleet-demo-123", "/tmp", "opencode")
	_ = c.Attach(ctx, "fleet-demo-123")
	_, _ = c.ListSessions(ctx)
	_ = c.KillSession(ctx, "fleet-demo-123")
	_, _ = c.CapturePane(ctx, "fleet-demo-123")
	want := [][]string{
		{"-L", "fleet", "new-session", "-d", "-s", "fleet-demo-123", "-c", "/tmp", "opencode"},
		{"-L", "fleet", "unbind-key", "-n", "-"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status", "on"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-style", "bg=colour235,fg=colour250"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-left-style", "bg=colour39,fg=colour235,bold"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-left", " Fleet #{session_name} "},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-right", " C-- back • C-g back "},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "window-status-current-style", "fg=colour212,bold"},
		{"-L", "fleet", "bind-key", "-n", "C-_", "detach-client"},
		{"-L", "fleet", "bind-key", "-n", "C-g", "detach-client"},
		{"-L", "fleet", "unbind-key", "-n", "-"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status", "on"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-style", "bg=colour235,fg=colour250"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-left-style", "bg=colour39,fg=colour235,bold"},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-left", " Fleet #{session_name} "},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "status-right", " C-- back • C-g back "},
		{"-L", "fleet", "set-option", "-t", "fleet-demo-123", "window-status-current-style", "fg=colour212,bold"},
		{"-L", "fleet", "bind-key", "-n", "C-_", "detach-client"},
		{"-L", "fleet", "bind-key", "-n", "C-g", "detach-client"},
		{"-L", "fleet", "attach", "-t", "fleet-demo-123"},
		{"-L", "fleet", "list-sessions", "-F", "#{session_name}"},
		{"-L", "fleet", "kill-session", "-t", "fleet-demo-123"},
		{"-L", "fleet", "capture-pane", "-t", "fleet-demo-123", "-p"},
	}
	if len(f.calls) != len(want) {
		t.Fatalf("calls=%d", len(f.calls))
	}
	for i := range want {
		if f.calls[i].name != "tmux" {
			t.Fatalf("call %d used %s", i, f.calls[i].name)
		}
		if !reflect.DeepEqual(f.calls[i].args, want[i]) {
			t.Fatalf("call %d args\n got %#v\nwant %#v", i, f.calls[i].args, want[i])
		}
	}
}

func TestRejectsEmptySocketAndNonFleetSessionName(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("expected empty socket error")
	}
	c := &Client{Socket: "fleet", Runner: &fakeRunner{}}
	if err := c.NewSession(context.Background(), "normal", "/tmp", "bash"); err == nil {
		t.Fatal("expected name prefix error")
	}
}

func TestAttachCommandUsesFleetSocket(t *testing.T) {
	c := &Client{Socket: "fleet-dev", Runner: &fakeRunner{}}
	cmd := c.AttachCommand("fleet-demo-123")
	want := []string{"tmux", "-L", "fleet-dev", "attach", "-t", "fleet-demo-123"}
	got := append([]string{filepath.Base(cmd.Path)}, cmd.Args[1:]...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestListNoServerIsEmpty(t *testing.T) {
	f := &fakeRunner{out: []byte("no server running on /tmp/tmux"), err: errors.New("exit 1")}
	c := &Client{Socket: "fleet-test", Runner: f}
	got, err := c.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestListMissingSocketIsEmpty(t *testing.T) {
	f := &fakeRunner{out: []byte("error connecting to /private/tmp/tmux-501/fleet-dev (No such file or directory)"), err: errors.New("exit 1")}
	c := &Client{Socket: "fleet-test", Runner: f}
	got, err := c.ListSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}
