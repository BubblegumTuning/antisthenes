package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
)

func TestAppendInputHistory_CapAndDedup(t *testing.T) {
	w := ChatWindow{}
	cfg := config.Config{InputHistoryEnabled: true, InputHistorySize: 3}

	w.appendInputHistory("a", cfg)
	w.appendInputHistory("b", cfg)
	w.appendInputHistory("c", cfg)
	w.appendInputHistory("d", cfg)

	if len(w.InputHistory) != 3 {
		t.Fatalf("history len = %d, want 3", len(w.InputHistory))
	}
	if w.InputHistory[0] != "b" || w.InputHistory[2] != "d" {
		t.Fatalf("unexpected cap order: %v", w.InputHistory)
	}

	w.appendInputHistory("d", cfg)
	if len(w.InputHistory) != 3 || w.HistoryIndex != 3 {
		t.Fatalf("dedup failed: %v idx=%d", w.InputHistory, w.HistoryIndex)
	}
}

func TestAppendInputHistory_Disabled(t *testing.T) {
	w := ChatWindow{}
	cfg := config.Config{InputHistoryEnabled: false, InputHistorySize: 50}
	w.appendInputHistory("hello", cfg)
	if len(w.InputHistory) != 0 {
		t.Error("history should stay empty when disabled")
	}
}

func TestInputHistoryFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hist.json")
	cfg := config.Config{
		InputHistoryEnabled: true,
		InputHistorySize:    5,
		InputHistoryPath:    path,
	}
	if err := saveInputHistoryFile(cfg, []string{"one", "two", "line\nwith\nbreaks"}); err != nil {
		t.Fatal(err)
	}
	got := loadInputHistoryFile(cfg)
	if len(got) != 3 || got[0] != "one" || got[2] != "line\nwith\nbreaks" {
		t.Fatalf("load = %#v", got)
	}
	// Cap on save.
	cfg.InputHistorySize = 2
	if err := saveInputHistoryFile(cfg, []string{"a", "b", "c"}); err != nil {
		t.Fatal(err)
	}
	got = loadInputHistoryFile(cfg)
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Fatalf("capped load = %#v", got)
	}
}

func TestRecordInputHistory_PersistsAndMirrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hist.json")
	m := Model{
		cfg: config.Config{
			InputHistoryEnabled: true,
			InputHistorySize:    50,
			InputHistoryPath:    path,
		},
		activeWindow: 0,
	}
	m.windows[0].SessionID = "a"
	m.windows[2].SessionID = "b"
	m.recordInputHistory("hello")
	if len(m.windows[0].InputHistory) != 1 || m.windows[0].InputHistory[0] != "hello" {
		t.Fatalf("active hist: %#v", m.windows[0].InputHistory)
	}
	if len(m.windows[2].InputHistory) != 1 || m.windows[2].InputHistory[0] != "hello" {
		t.Fatalf("mirrored hist: %#v", m.windows[2].InputHistory)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("file missing entry: %s", data)
	}
	// Seed from disk into a fresh model.
	m2 := Model{
		cfg: m.cfg,
	}
	m2.windows[0].SessionID = "z"
	m2.seedInputHistoryFromDisk()
	if len(m2.windows[0].InputHistory) != 1 || m2.windows[0].InputHistory[0] != "hello" {
		t.Fatalf("seed: %#v", m2.windows[0].InputHistory)
	}
}

func TestUpdate_ClearHistorySlash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hist.json")
	cfg := config.Config{
		AgentName:           "Test",
		InputHistoryEnabled: true,
		InputHistorySize:    50,
		InputHistoryPath:    path,
	}
	if err := saveInputHistoryFile(cfg, []string{"one", "two"}); err != nil {
		t.Fatal(err)
	}
	ti := textarea.New()
	ti.SetValue("/clear-history")
	m := Model{
		textInput: ti,
		ready:     true,
		width:     80,
		viewport:  viewport.New(80, 20),
		cfg:       cfg,
	}
	m.windows[0].SessionID = "s"
	m.windows[0].InputHistory = []string{"one", "two"}
	m.windows[0].HistoryIndex = 2
	m.windows[2].SessionID = "t"
	m.windows[2].InputHistory = []string{"one", "two"}

	updated, _ := modelFromUpdate(&m, tea.KeyMsg{Type: tea.KeyEnter})
	if len(updated.windows[0].InputHistory) != 0 {
		t.Error("history should be cleared on active window")
	}
	if len(updated.windows[2].InputHistory) != 0 {
		t.Error("history should be cleared on other windows")
	}
	last := updated.windows[0].Messages[len(updated.windows[0].Messages)-1]
	if !strings.Contains(last.Content, "Input history cleared") {
		t.Errorf("unexpected ack: %q", last.Content)
	}
	got := loadInputHistoryFile(cfg)
	if len(got) != 0 {
		t.Fatalf("file should be empty after clear, got %#v", got)
	}
}

func TestUpdate_KeyUp_DisabledByConfig(t *testing.T) {
	w := ChatWindow{InputHistory: []string{"prev"}, HistoryIndex: 1}
	m := Model{
		cfg:       config.Config{InputHistoryEnabled: false},
		textInput: textarea.New(),
		ready:     true,
	}
	m.windows[0] = w
	m.textInput.SetValue("")

	_, handled := (&m).handleKeyMsg(tea.KeyMsg{Type: tea.KeyUp})
	if handled {
		t.Error("Up should not be handled when history disabled")
	}
}

func TestConfig_InputHistoryFile(t *testing.T) {
	cfg := config.Config{WorkDir: "/tmp/wd"}
	if got := cfg.InputHistoryFile(); got != filepath.Join("/tmp/wd", "input_history.json") {
		t.Fatalf("default path = %q", got)
	}
	cfg.InputHistoryPath = "/custom/h.json"
	if got := cfg.InputHistoryFile(); got != "/custom/h.json" {
		t.Fatalf("override = %q", got)
	}
}
