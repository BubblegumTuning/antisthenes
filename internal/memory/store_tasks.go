package memory

import "time"

func (s *Store) SaveTask(id, schedule, command string, nextRun time.Time) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO scheduled_tasks (id, schedule, command, next_run)
		VALUES (?, ?, ?, ?)`, id, schedule, command, nextRun.Unix())
	return err
}

func (s *Store) LoadTasks() (map[string]struct {
	Schedule string
	Command  string
	NextRun  time.Time
}, error) {
	rows, err := s.db.Query(`SELECT id, schedule, command, next_run FROM scheduled_tasks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make(map[string]struct {
		Schedule string
		Command  string
		NextRun  time.Time
	})

	for rows.Next() {
		var id, schedule, command string
		var nextRunUnix int64
		if err := rows.Scan(&id, &schedule, &command, &nextRunUnix); err != nil {
			return nil, err
		}
		tasks[id] = struct {
			Schedule string
			Command  string
			NextRun  time.Time
		}{
			Schedule: schedule,
			Command:  command,
			NextRun:  time.Unix(nextRunUnix, 0),
		}
	}
	return tasks, nil
}

func (s *Store) DeleteTask(id string) error {
	_, err := s.db.Exec(`DELETE FROM scheduled_tasks WHERE id = ?`, id)
	return err
}
