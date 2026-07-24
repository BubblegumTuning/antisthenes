package memory

import (
	"fmt"
	"time"
)

// SessionInfo is a catalogue row for listing/resume UX.
type SessionInfo struct {
	ID        string
	Title     string
	CreatedAt int64
	UpdatedAt int64
}

func (s *Store) CreateSession() (string, error) {
	id := fmt.Sprintf("sess-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano())
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO sessions (id, created_at, updated_at, title) VALUES (?, ?, ?, ?)`, id, now, now, "")
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
	infos, err := s.ListSessionInfos(limit)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(infos))
	for _, info := range infos {
		ids = append(ids, info.ID)
	}
	return ids, nil
}

// ListSessionInfos returns recent sessions ordered by updated_at DESC.
func (s *Store) ListSessionInfos(limit int) ([]SessionInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`
		SELECT id, COALESCE(title, ''), created_at, updated_at
		FROM sessions
		ORDER BY updated_at DESC, created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SessionInfo
	for rows.Next() {
		var info SessionInfo
		if err := rows.Scan(&info.ID, &info.Title, &info.CreatedAt, &info.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, info)
	}
	return out, nil
}

// SetSessionTitle stores a display title for the session.
func (s *Store) SetSessionTitle(sessionID, title string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE sessions SET title = ?, updated_at = ? WHERE id = ?`, title, now, sessionID)
	return err
}

// GetSessionTitle returns the stored title (may be empty).
func (s *Store) GetSessionTitle(sessionID string) (string, error) {
	var title string
	err := s.db.QueryRow(`SELECT COALESCE(title, '') FROM sessions WHERE id = ?`, sessionID).Scan(&title)
	if err != nil {
		return "", err
	}
	return title, nil
}

// LoadChatMessages returns structured messages for restoring a TUI/agent session.
func (s *Store) LoadChatMessages(sessionID string) ([]struct {
	Role       string
	Content    string
	ToolCallID string
}, error,
) {
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

// ClearSessionMessages removes all persisted messages for a session and matching FTS rows.
// Title is left intact unless ClearSessionTitle is used separately.
func (s *Store) ClearSessionMessages(sessionID string) error {
	if _, err := s.db.Exec(`DELETE FROM messages WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	// Standalone FTS5 table (not external-content): must delete explicitly or search keeps ghosts.
	if _, err := s.db.Exec(`DELETE FROM messages_fts WHERE session_id = ?`, sessionID); err != nil {
		return err
	}
	return nil
}

// ClearSessionTitle blanks the session title (e.g. after /clear for a fresh topic).
func (s *Store) ClearSessionTitle(sessionID string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`UPDATE sessions SET title = '', updated_at = ? WHERE id = ?`, now, sessionID)
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
