package memory

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store provides persistent memory with FTS5 and nudges.
type Store struct {
	db *sql.DB
}

// NewStore opens or creates the SQLite DB.
func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			created_at INTEGER,
			updated_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			role TEXT,
			content TEXT,
			created_at INTEGER,
			FOREIGN KEY(session_id) REFERENCES sessions(id)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(content, session_id UNINDEXED)`,
		`CREATE TABLE IF NOT EXISTS nudges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT,
			source TEXT,
			content TEXT,
			created_at INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			schedule TEXT,
			command TEXT,
			next_run INTEGER
		)`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return nil, fmt.Errorf("init query failed: %w", err)
		}
	}

	s := &Store{db: db}
	s.migrate()
	return s, nil
}

func (s *Store) migrate() {
	// Best-effort schema upgrades for existing databases.
	_, _ = s.db.Exec(`ALTER TABLE messages ADD COLUMN tool_call_id TEXT NOT NULL DEFAULT ''`)
}

func (s *Store) Close() error {
	return s.db.Close()
}
