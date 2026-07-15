package memory

import "time"

func (s *Store) AddNudge(sessionID, source, content string) error {
	now := time.Now().Unix()
	_, err := s.db.Exec(`INSERT INTO nudges (session_id, source, content, created_at) VALUES (?, ?, ?, ?)`,
		sessionID, source, content, now)
	return err
}

func (s *Store) GetRecentNudges(sessionID string, limit int) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT source || ': ' || content 
		FROM nudges 
		WHERE session_id = ? 
		ORDER BY created_at DESC 
		LIMIT ?`, sessionID, limit)
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
