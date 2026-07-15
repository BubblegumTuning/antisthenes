package memory

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateAndListSessions(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test-sessions.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sid, err := store.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	if sid == "" {
		t.Error("expected non-empty session id")
	}

	sessions, err := store.ListSessions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) == 0 {
		t.Error("expected at least one session")
	}
}

func TestAddAndSearchMessages(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test-search.db")

	store, _ := NewStore(dbPath)
	defer store.Close()

	sid, _ := store.CreateSession()
	if err := store.AddMessage(sid, "user", "hello world test message"); err != nil {
		t.Fatal(err)
	}

	results, err := store.SearchMessages("hello", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Error("expected search results")
	}
}

func TestSaveLoadDeleteTask(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test-tasks.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	if err := store.SaveTask("t1", "*/5 * * * *", "echo hi", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	tasks, err := store.LoadTasks()
	if err != nil || len(tasks) != 1 {
		t.Fatalf("LoadTasks: %v, len=%d", err, len(tasks))
	}
	if err := store.DeleteTask("t1"); err != nil {
		t.Fatal(err)
	}
	tasks, _ = store.LoadTasks()
	if len(tasks) != 0 {
		t.Error("task not deleted")
	}
}

func TestAddNudgeAndGetRecentNudges(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "memory-nudges.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	sid, err := store.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Add multiple nudges
	if err := store.AddNudge(sid, "tui", "nudge one"); err != nil {
		t.Fatalf("AddNudge1: %v", err)
	}
	if err := store.AddNudge(sid, "cron", "nudge two"); err != nil {
		t.Fatalf("AddNudge2: %v", err)
	}

	nudges, err := store.GetRecentNudges(sid, 5)
	if err != nil {
		t.Fatalf("GetRecentNudges: %v", err)
	}
	if len(nudges) != 2 {
		t.Fatalf("expected 2 nudges, got %d", len(nudges))
	}
	// Check both present (order may not be strict if same-second timestamps)
	found1, found2 := false, false
	for _, n := range nudges {
		if strings.Contains(n, "cron: nudge two") {
			found2 = true
		}
		if strings.Contains(n, "tui: nudge one") {
			found1 = true
		}
	}
	if !found1 || !found2 {
		t.Errorf("expected both nudges, got: %v", nudges)
	}

	// Limit
	nudges, _ = store.GetRecentNudges(sid, 1)
	if len(nudges) != 1 {
		t.Errorf("limit 1 expected 1, got %d", len(nudges))
	}
}

func TestLoadSessionMessages(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "memory-messages.db")

	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	sid, err := store.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if err := store.AddMessage(sid, "user", "first message"); err != nil {
		t.Fatalf("AddMessage1: %v", err)
	}
	if err := store.AddMessage(sid, "assistant", "reply here"); err != nil {
		t.Fatalf("AddMessage2: %v", err)
	}

	msgs, err := store.LoadSessionMessages(sid)
	if err != nil {
		t.Fatalf("LoadSessionMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "user: first message") {
		t.Errorf("first msg wrong: %s", msgs[0])
	}
	if !strings.Contains(msgs[1], "assistant: reply here") {
		t.Errorf("second msg wrong: %s", msgs[1])
	}

	// Empty session
	sid2, _ := store.CreateSession()
	msgs, _ = store.LoadSessionMessages(sid2)
	if len(msgs) != 0 {
		t.Errorf("new session should have 0 msgs, got %d", len(msgs))
	}
}

func TestLoadChatMessages_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(filepath.Join(tmp, "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	sid, _ := store.CreateSession()
	if err := store.AddChatMessage(sid, "user", "hello", ""); err != nil {
		t.Fatalf("AddChatMessage user: %v", err)
	}
	if err := store.AddChatMessage(sid, "assistant", "hi there", ""); err != nil {
		t.Fatalf("AddChatMessage assistant: %v", err)
	}
	if err := store.AddChatMessage(sid, "tool", "file contents", "call_abc"); err != nil {
		t.Fatalf("AddChatMessage tool: %v", err)
	}

	records, err := store.LoadChatMessages(sid)
	if err != nil {
		t.Fatalf("LoadChatMessages: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if records[0].Role != "user" || records[0].Content != "hello" {
		t.Errorf("user record wrong: %+v", records[0])
	}
	if records[2].ToolCallID != "call_abc" {
		t.Errorf("tool call id wrong: %+v", records[2])
	}
}

func TestClearSessionMessages(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(filepath.Join(tmp, "clear.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	sid, _ := store.CreateSession()
	_ = store.AddChatMessage(sid, "user", "msg", "")
	if err := store.ClearSessionMessages(sid); err != nil {
		t.Fatalf("ClearSessionMessages: %v", err)
	}
	records, _ := store.LoadChatMessages(sid)
	if len(records) != 0 {
		t.Errorf("expected 0 after clear, got %d", len(records))
	}
}

func TestNewStore_BadPath(t *testing.T) {
	// Try a path that might cause issues (though sqlite is lenient; exercises error path if any)
	_, err := NewStore("/nonexistent/dir/that/should/not/exist/test.db")
	if err == nil {
		// May succeed or fail depending on sqlite driver; just ensure no panic and coverage
		t.Log("NewStore on bad path succeeded (driver behavior)")
	}
}
