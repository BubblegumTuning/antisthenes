package tui

import (
	"path/filepath"
	"testing"

	"github.com/nanami/antisthenes/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

func TestLoadSessionFromStore(t *testing.T) {
	tmp := t.TempDir()
	store, err := memory.NewStore(filepath.Join(tmp, "tui-persist.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	sid, err := store.CreateSession()
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.AddChatMessage(sid, "user", "prior message", ""); err != nil {
		t.Fatalf("AddChatMessage: %v", err)
	}
	if err := store.AddNudge(sid, "cron", "task done"); err != nil {
		t.Fatalf("AddNudge: %v", err)
	}

	m := Model{store: store}
	m.windows[0] = ChatWindow{SessionID: sid}
	m.loadWindowFromStore(0)

	if len(m.windows[0].Messages) != 1 || m.windows[0].Messages[0].Content != "prior message" {
		t.Fatalf("messages not restored: %+v", m.windows[0].Messages)
	}
	if m.windows[0].PersistedMsgCount != 1 {
		t.Errorf("persistedMsgCount = %d, want 1", m.windows[0].PersistedMsgCount)
	}
	if len(m.windows[0].Nudges) != 1 || m.windows[0].Nudges[0] != "cron: task done" {
		t.Errorf("nudges not restored: %v", m.windows[0].Nudges)
	}
}

func TestPersistAndClearSessionMemory(t *testing.T) {
	tmp := t.TempDir()
	store, err := memory.NewStore(filepath.Join(tmp, "tui-persist2.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	sid, _ := store.CreateSession()
	m := Model{store: store}
	m.windows[0] = ChatWindow{
		SessionID: sid,
		Messages: []openai.ChatCompletionMessage{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
	}
	m.persistNewMessages()

	records, err := store.LoadChatMessages(sid)
	if err != nil || len(records) != 2 {
		t.Fatalf("persist failed: %v len=%d", err, len(records))
	}
	if m.windows[0].PersistedMsgCount != 2 {
		t.Errorf("persistedMsgCount = %d, want 2", m.windows[0].PersistedMsgCount)
	}

	m.clearSessionMemory()
	records, _ = store.LoadChatMessages(sid)
	if len(records) != 0 || len(m.windows[0].Messages) != 0 {
		t.Errorf("clear failed: db=%d mem=%d", len(records), len(m.windows[0].Messages))
	}
}
