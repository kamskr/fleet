package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kamskr/fleet/internal/session"
)

func TestJSONStoreMissingFileLoadsEmpty(t *testing.T) {
	s := NewJSON(filepath.Join(t.TempDir(), "missing", "state.json"))
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d sessions", len(got))
	}
}

func TestJSONStoreSaveLoadRoundTrip(t *testing.T) {
	s := NewJSON(filepath.Join(t.TempDir(), "state.json"))
	want := session.New("Work", "/tmp", "opencode")
	if err := s.Save([]session.Session{want}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != want.ID || got[0].DisplayName != "Work" {
		t.Fatalf("bad roundtrip: %#v", got)
	}
}

func TestJSONStoreUpsertAndDelete(t *testing.T) {
	s := NewJSON(filepath.Join(t.TempDir(), "nested", "state.json"))
	one := session.New("One", "/tmp", "bash")
	if err := s.Upsert(one); err != nil {
		t.Fatal(err)
	}
	one.DisplayName = "Two"
	if err := s.Upsert(one); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].DisplayName != "Two" {
		t.Fatalf("bad upsert: %#v", got)
	}
	if err := s.Delete(one.ID); err != nil {
		t.Fatal(err)
	}
	got, err = s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("delete failed: %#v", got)
	}
}

func TestJSONStoreCorruptJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(p, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewJSON(p).Load(); err == nil {
		t.Fatal("expected error")
	}
}
