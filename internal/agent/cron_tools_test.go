package agent

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanami/antisthenes/internal/cron"
	"github.com/nanami/antisthenes/internal/memory"
)

func TestCronTools_NilScheduler(t *testing.T) {
	r := NewToolRegistry()
	RegisterCronTools(r, nil)

	res, err := r.Call("schedule_task", map[string]any{"id": "t", "schedule": "every 1m", "command": "hi"})
	if err != nil || !strings.Contains(res, "not active") {
		t.Fatalf("nil sched: %v %s", err, res)
	}
}

func TestCronTools_ScheduleListCancel(t *testing.T) {
	tmp := t.TempDir()
	store, err := memory.NewStore(filepath.Join(tmp, "cron.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sched := cron.NewScheduler(store)
	defer sched.Stop()

	r := NewToolRegistry()
	RegisterCronTools(r, sched)

	res, err := r.Call("schedule_task", map[string]any{
		"id":       "daily-check",
		"schedule": "every 10m",
		"command":  "check disk usage",
	})
	if err != nil || !strings.Contains(res, "scheduled") {
		t.Fatalf("schedule: %v %s", err, res)
	}

	res, err = r.Call("list_tasks", map[string]any{})
	if err != nil || !strings.Contains(res, "daily-check") {
		t.Fatalf("list: %v %s", err, res)
	}

	res, err = r.Call("cancel_task", map[string]any{"id": "daily-check"})
	if err != nil || !strings.Contains(res, "cancelled") {
		t.Fatalf("cancel: %v %s", err, res)
	}

	res, err = r.Call("list_tasks", map[string]any{})
	if err != nil || !strings.Contains(res, "no scheduled tasks") {
		t.Fatalf("list after cancel: %v %s", err, res)
	}

	// persisted round-trip
	sched2 := cron.NewScheduler(store)
	defer sched2.Stop()
	time.Sleep(10 * time.Millisecond)
	r2 := NewToolRegistry()
	RegisterCronTools(r2, sched2)
	res, err = r2.Call("list_tasks", map[string]any{})
	if err != nil || strings.Contains(res, "daily-check") {
		t.Fatalf("should not persist cancelled task: %v %s", err, res)
	}
}
