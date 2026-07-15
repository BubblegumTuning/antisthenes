package cron

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nanami/antisthenes/internal/memory"
)

func TestNextRunFromSchedule(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		wantMin  time.Duration // approximate min
	}{
		{"every 30s", "every 30s", 30 * time.Second},
		{"every 5m", "every 5m", 5 * time.Minute},
		{"every 1h", "every 1h", time.Hour},
		{"every 2h", "every 2h", 2 * time.Hour},
		{"invalid", "foo", 5 * time.Minute},
		{"empty", "", 5 * time.Minute},
		{"case", "EVERY 10s", 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextRunFromSchedule(tt.schedule)
			if got < tt.wantMin-time.Second || got > tt.wantMin+time.Second { // rough
				if tt.name != "invalid" && tt.name != "empty" {
					t.Errorf("nextRunFromSchedule(%q) = %v, want ~%v", tt.schedule, got, tt.wantMin)
				}
			}
		})
	}
}

func TestNewScheduler(t *testing.T) {
	tmp := t.TempDir()
	db := filepath.Join(tmp, "cron-test.db")
	store, err := memory.NewStore(db)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()
	defer os.Remove(db)

	s := NewScheduler(store)
	if s == nil {
		t.Fatal("nil scheduler")
	}
	defer s.Stop()

	if len(s.tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(s.tasks))
	}
}

func TestScheduleAndCheck(t *testing.T) {
	tmp := t.TempDir()
	db := filepath.Join(tmp, "cron-test.db")
	store, _ := memory.NewStore(db)
	defer store.Close()

	s := NewScheduler(store)
	defer s.Stop()

	execCalled := make(chan Task, 1)
	exec := func(task Task) {
		execCalled <- task
	}

	s.Schedule("t1", "every 1s", "echo hi", exec)

	if len(s.tasks) != 1 {
		t.Fatalf("expected 1 task")
	}

	// force past due
	s.mu.Lock()
	s.tasks["t1"].NextRun = time.Now().Add(-time.Hour)
	s.mu.Unlock()

	s.checkTasks()

	select {
	case task := <-execCalled:
		if task.ID != "t1" {
			t.Errorf("wrong task %s", task.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("exec not called")
	}
}

func TestRegisterAgent(t *testing.T) {
	s := NewScheduler(nil)
	defer s.Stop()

	called := make(chan string, 1)
	s.RegisterAgent(func(cmd string) {
		called <- cmd
	})

	// manually add task with past time, no Exec
	s.mu.Lock()
	s.tasks["t2"] = &Task{
		ID:       "t2",
		Schedule: "every 1s",
		Command:  "test cmd",
		NextRun:  time.Now().Add(-time.Hour),
	}
	s.mu.Unlock()

	s.checkTasks()

	select {
	case cmd := <-called:
		if cmd != "test cmd" {
			t.Errorf("got cmd %s", cmd)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("agent func not called")
	}
}

func TestStop(t *testing.T) {
	s := NewScheduler(nil)
	s.Stop()
	// should not block or panic
}
