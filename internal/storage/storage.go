package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Status string

const (
	StatusTodo       Status = "todo"
	StatusAttempted  Status = "attempted"
	StatusSolved     Status = "solved"
	StatusRevisiting Status = "revisiting"
	StatusSkipped    Status = "skipped"
)

var orderedStatuses = []Status{
	StatusTodo,
	StatusAttempted,
	StatusSolved,
	StatusRevisiting,
	StatusSkipped,
}

func Statuses() []Status {
	out := make([]Status, len(orderedStatuses))
	copy(out, orderedStatuses)
	return out
}

func NextStatus(status Status) Status {
	for i, candidate := range orderedStatuses {
		if candidate == status {
			return orderedStatuses[(i+1)%len(orderedStatuses)]
		}
	}
	return StatusAttempted
}

func ValidStatus(status Status) bool {
	for _, candidate := range orderedStatuses {
		if candidate == status {
			return true
		}
	}
	return false
}

type ProgressStore interface {
	Progress(ctx context.Context, slug string) (Status, error)
	SetProgress(ctx context.Context, slug string, status Status) error
	Close() error
}

type SQLiteStore struct {
	db *sql.DB
}

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.init(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS problem_progress (
	slug TEXT PRIMARY KEY,
	status TEXT NOT NULL CHECK (status IN ('todo', 'attempted', 'solved', 'revisiting', 'skipped')),
	updated_at TEXT NOT NULL
);`)
	return err
}

func (s *SQLiteStore) Progress(ctx context.Context, slug string) (Status, error) {
	var status Status
	err := s.db.QueryRowContext(ctx, `SELECT status FROM problem_progress WHERE slug = ?`, slug).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return StatusTodo, nil
	}
	if err != nil {
		return "", err
	}
	if !ValidStatus(status) {
		return "", fmt.Errorf("invalid stored status %q for %s", status, slug)
	}
	return status, nil
}

func (s *SQLiteStore) SetProgress(ctx context.Context, slug string, status Status) error {
	if !ValidStatus(status) {
		return fmt.Errorf("invalid progress status %q", status)
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO problem_progress (slug, status, updated_at)
VALUES (?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at`,
		slug, status, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
