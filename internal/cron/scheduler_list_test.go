package cron

import "testing"

func TestListAndCancel(t *testing.T) {
	s := NewScheduler(nil)
	defer s.Stop()

	s.Schedule("a", "every 1m", "cmd a", nil)
	s.Schedule("b", "every 2m", "cmd b", nil)

	list := s.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}

	if !s.Cancel("a") {
		t.Fatal("cancel a failed")
	}
	if s.Cancel("missing") {
		t.Fatal("cancel missing should fail")
	}
	list = s.List()
	if len(list) != 1 || list[0].ID != "b" {
		t.Fatalf("after cancel: %+v", list)
	}
	if list[0].NextRun.IsZero() {
		t.Fatal("next run should be set")
	}
}
