package memory

import (
	"fmt"
	"time"
)

func (s *Store) CreateSession() (string, error) {
	id := fmt.Sprintf("sess-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano())
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO sessions (id, created_at, updated_at) VALUES (?, ?, ?)`, id, now, now)
	return id, err
}

func (s *Store) AddMessage(sessionID, role, content string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, role, content, now)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO messages_fts (content, session_id) VALUES (?, ?)`, content, sessionID)
	return err
}

func (s *Store) SearchMessages(query string, limit int) ([]string, error) {
	rows, err := s.db.Query(`SELECT content FROM messages_fts WHERE content MATCH ? LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		results = append(results, c)
	}
	return results, nil
}

func (s *Store) ListSessions(limit int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT id FROM sessions 
		ORDER BY created_at DESC 
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// LoadChatMessages returns structured messages for restoring a TUI/agent session.
func (s *Store) LoadChatMessages(sessionID string) ([]struct {
	Role       string
	Content    string
	ToolCallID string
}, error) {
	rows, err := s.db.Query(`
		SELECT role, content, tool_call_id
		FROM messages
		WHERE session_id = ?
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []struct {
		Role       string
		Content    string
		ToolCallID string
	}
	for rows.Next() {
		var rec struct {
			Role       string
			Content    string
			ToolCallID string
		}
		if err := rows.Scan(&rec.Role, &rec.Content, &rec.ToolCallID); err != nil {
			return nil, err
		}
		results = append(results, rec)
	}
	return results, nil
}

// AddChatMessage persists a single chat message (user, assistant, system, or tool).
// Messages with empty content and no tool-call payload are skipped.
func (s *Store) AddChatMessage(sessionID, role, content, toolCallID string) error {
	if content == "" {
		return nil
	}
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO messages (session_id, role, content, tool_call_id, created_at) VALUES (?, ?, ?, ?, ?)`,
		sessionID, role, content, toolCallID, now)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO messages_fts (content, session_id) VALUES (?, ?)`, content, sessionID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE sessions SET updated_at = ? WHERE id = ?`, now, sessionID)
	return err
}

// ClearSessionMessages removes all persisted messages for a session.
func (s *Store) ClearSessionMessages(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID)
	return err
}

func (s *Store) LoadSessionMessages(sessionID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT role || ': ' || content 
		FROM messages 
		WHERE session_id = ? 
		ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, err
		}
		results = append(results, line)
	}
	return results, nil
}
