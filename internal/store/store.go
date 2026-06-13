package store

import "github.com/kamskr/fleet/internal/session"

type Store interface {
	Load() ([]session.Session, error)
	Save([]session.Session) error
	Upsert(session.Session) error
	Delete(id string) error
}
