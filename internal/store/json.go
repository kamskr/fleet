package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/kamskr/fleet/internal/session"
)

type JSONStore struct{ Path string }

type stateFile struct {
	Version  int               `json:"version"`
	Sessions []session.Session `json:"sessions"`
}

func NewJSON(path string) *JSONStore { return &JSONStore{Path: path} }

func (s *JSONStore) Load() ([]session.Session, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return []session.Session{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return []session.Session{}, nil
	}
	var st stateFile
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	if st.Sessions == nil {
		st.Sessions = []session.Session{}
	}
	return st.Sessions, nil
}

func (s *JSONStore) Save(sessions []session.Session) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(stateFile{Version: 1, Sessions: sessions}, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.Path)
}

func (s *JSONStore) Upsert(next session.Session) error {
	sessions, err := s.Load()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	found := false
	for i := range sessions {
		if sessions[i].ID == next.ID {
			next.CreatedAt = sessions[i].CreatedAt
			next.UpdatedAt = now
			sessions[i] = next
			found = true
			break
		}
	}
	if !found {
		if next.CreatedAt.IsZero() {
			next.CreatedAt = now
		}
		next.UpdatedAt = now
		sessions = append(sessions, next)
	}
	return s.Save(sessions)
}

func (s *JSONStore) Delete(id string) error {
	sessions, err := s.Load()
	if err != nil {
		return err
	}
	out := sessions[:0]
	for _, sess := range sessions {
		if sess.ID != id {
			out = append(out, sess)
		}
	}
	return s.Save(out)
}
