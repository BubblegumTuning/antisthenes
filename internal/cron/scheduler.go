package cron

import (
	"strings"
	"sync"
	"time"

	"github.com/nanami/antisthenes/internal/memory"
)

// Task represents a scheduled task.
type Task struct {
	ID       string
	Schedule string
	Command  string
	NextRun  time.Time
	Exec     func(task Task)
}

// Scheduler manages simple recurring tasks with persistence.
type Scheduler struct {
	tasks     map[string]*Task
	store     *memory.Store
	mu        sync.Mutex
	ticker    *time.Ticker
	stop      chan struct{}
	agentFunc func(command string)
}

// NewScheduler creates a new scheduler with persistence.
func NewScheduler(store *memory.Store) *Scheduler {
	s := &Scheduler{
		tasks: make(map[string]*Task),
		store: store,
		stop:  make(chan struct{}),
	}
	s.ticker = time.NewTicker(30 * time.Second)

	if store != nil {
		persisted, _ := store.LoadTasks()
		for id, t := range persisted {
			s.tasks[id] = &Task{
				ID:       id,
				Schedule: t.Schedule,
				Command:  t.Command,
				NextRun:  t.NextRun,
			}
		}
	}

	go s.run()
	return s
}

func (s *Scheduler) run() {
	for {
		select {
		case <-s.ticker.C:
			s.checkTasks()
		case <-s.stop:
			return
		}
	}
}

func (s *Scheduler) checkTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, t := range s.tasks {
		if now.After(t.NextRun) {
			if t.Exec != nil {
				go t.Exec(*t)
			} else if s.agentFunc != nil {
				go s.agentFunc(t.Command)
			}
			interval := nextRunFromSchedule(t.Schedule)
			t.NextRun = now.Add(interval)

			if s.store != nil {
				_ = s.store.SaveTask(t.ID, t.Schedule, t.Command, t.NextRun)
			}
		}
	}
}

// nextRunFromSchedule calculates the next run time from a simple schedule string.
// Supports: "every 30s", "every 5m", "every 1h", "every 2h"
func nextRunFromSchedule(schedule string) time.Duration {
	schedule = strings.ToLower(strings.TrimSpace(schedule))
	if strings.HasPrefix(schedule, "every ") {
		parts := strings.Fields(schedule)
		if len(parts) == 2 {
			durStr := parts[1]
			if d, err := time.ParseDuration(durStr); err == nil {
				return d
			}
		}
	}
	// Fallback
	return 5 * time.Minute
}

// Schedule adds a new task.
func (s *Scheduler) Schedule(id, schedule, command string, exec func(task Task)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	interval := nextRunFromSchedule(schedule)
	task := &Task{
		ID:       id,
		Schedule: schedule,
		Command:  command,
		NextRun:  time.Now().Add(interval),
		Exec:     exec,
	}
	s.tasks[id] = task
	if s.store != nil {
		_ = s.store.SaveTask(id, schedule, command, task.NextRun)
	}
}

// RegisterAgent allows wiring the scheduler to an agent callback
// so tasks can execute real agent work.
func (s *Scheduler) RegisterAgent(agentFunc func(command string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentFunc = agentFunc
}

// List returns a snapshot of scheduled tasks.
func (s *Scheduler) List() []Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		cp := *t
		cp.Exec = nil
		out = append(out, cp)
	}
	return out
}

// Cancel removes a task by id.
func (s *Scheduler) Cancel(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[id]; !ok {
		return false
	}
	delete(s.tasks, id)
	if s.store != nil {
		_ = s.store.DeleteTask(id)
	}
	return true
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.ticker.Stop()
	close(s.stop)
}
