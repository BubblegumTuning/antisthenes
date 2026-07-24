package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
)

func TestReadFileFromOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "work.log")
	if err := os.WriteFile(path, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chunk, off, err := readFileFromOffset(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(chunk) != "line1\nline2\n" || off != int64(len(chunk)) {
		t.Fatalf("full read chunk=%q off=%d", chunk, off)
	}
	// no new data
	chunk2, off2, err := readFileFromOffset(path, off)
	if err != nil || len(chunk2) != 0 || off2 != off {
		t.Fatalf("idle read chunk=%q off=%d err=%v", chunk2, off2, err)
	}
	// append
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("line3\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	chunk3, off3, err := readFileFromOffset(path, off)
	if err != nil {
		t.Fatal(err)
	}
	if string(chunk3) != "line3\n" {
		t.Fatalf("tail=%q", chunk3)
	}
	if off3 != off+int64(len(chunk3)) {
		t.Fatalf("off3=%d", off3)
	}
	// truncate restart
	if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	chunk4, off4, err := readFileFromOffset(path, off3)
	if err != nil {
		t.Fatal(err)
	}
	if string(chunk4) != "new\n" || off4 != 4 {
		t.Fatalf("after truncate chunk=%q off=%d", chunk4, off4)
	}
}

func TestHandleIterativeLogProgress_AppliesAndStale(t *testing.T) {
	m := Model{
		ready:    true,
		viewport: viewport.New(80, 20),
		cfg:      config.Config{AutoScroll: true},
	}
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterGen = 2
	m.handleIterativeLogProgress(iterativeLogProgressMsg{
		win: 0,
		gen: 1, chunk: "stale\n", newOffset: 10,
	})
	if m.windows[0].iterLogOffset != 0 || m.windows[0].iterProgressSnippet != "" || len(m.windows[0].Messages) != 0 {
		t.Fatalf("stale should be ignored: offset=%d snip=%q msgs=%d", m.windows[0].iterLogOffset, m.windows[0].iterProgressSnippet, len(m.windows[0].Messages))
	}

	m.handleIterativeLogProgress(iterativeLogProgressMsg{
		win: 0,
		gen: 2, chunk: "step A done\nstep B done\n", newOffset: 24,
	})
	if m.windows[0].iterLogOffset != 24 {
		t.Fatalf("offset=%d", m.windows[0].iterLogOffset)
	}
	if m.windows[0].iterProgressSnippet != "step B done" {
		t.Fatalf("snippet=%q", m.windows[0].iterProgressSnippet)
	}
	if len(m.windows[0].Messages) != 1 || !strings.Contains(m.windows[0].Messages[0].Content, "[iterative log]") {
		t.Fatalf("chat progress missing: %+v", m.windows[0].Messages)
	}
	if !strings.Contains(m.windows[0].Messages[0].Content, "step A done") {
		t.Fatalf("chunk body missing: %s", m.windows[0].Messages[0].Content)
	}
	// empty chunk only advances offset
	before := len(m.windows[0].Messages)
	m.handleIterativeLogProgress(iterativeLogProgressMsg{win: 0, gen: 2, chunk: "\n", newOffset: 25})
	if m.windows[0].iterLogOffset != 25 || len(m.windows[0].Messages) != before {
		t.Fatalf("empty chunk should not append: offset=%d msgs=%d", m.windows[0].iterLogOffset, len(m.windows[0].Messages))
	}
}

func TestHandleIterativeLogTick_SchedulesReadWhenExecuting(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "p.log")
	if err := os.WriteFile(logPath, []byte("hello log\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := Model{
		ready:    true,
		viewport: viewport.New(40, 10),
	}
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterGen = 3
	m.windows[0].iterCtx = IterativeContext{LogFile: logPath}
	_, cmd := m.handleIterativeLogTick(iterativeLogTickMsg{win: 0, gen: 3})
	if cmd == nil {
		t.Fatal("expected cmd batch")
	}
	// Drain batch: should produce progress msg (and a tick we ignore)
	deadline := time.Now().Add(2 * time.Second)
	results := make(chan tea.Msg, 8)
	var run func(tea.Cmd)
	run = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, bc := range batch {
				go run(bc)
			}
			return
		}
		results <- msg
	}
	go run(cmd)
	var sawProgress bool
	for time.Now().Before(deadline) {
		select {
		case msg := <-results:
			if p, ok := msg.(iterativeLogProgressMsg); ok {
				if p.gen != 3 {
					t.Fatalf("gen=%d", p.gen)
				}
				if !strings.Contains(p.chunk, "hello log") {
					t.Fatalf("chunk=%q", p.chunk)
				}
				_ = true // progress received
				return
			}
		case <-time.After(20 * time.Millisecond):
		}
	}
	if !sawProgress {
		t.Fatal("did not receive iterativeLogProgressMsg")
	}
}

func TestFormatIterativeLogProgressAndLabels(t *testing.T) {
	if !strings.HasPrefix(formatIterativeLogProgress("x"), "[iterative log]\n") {
		t.Fatal(formatIterativeLogProgress("x"))
	}
	m := Model{}
	m.windows[0].iterState = IterExecuting
	m.windows[0].iterProgressSnippet = "built pkg"
	if !strings.Contains(m.showIterativeStatus(), "built pkg") {
		t.Fatal(m.showIterativeStatus())
	}
	if !strings.Contains(m.iterativeThinkingLabel(), "built pkg") {
		t.Fatal(m.iterativeThinkingLabel())
	}
	m2 := Model{}
	m2.windows[0].iterState = IterPlanning
	if m2.iterativeThinkingLabel() != " Thinking..." {
		t.Fatal(m2.iterativeThinkingLabel())
	}
}

func TestTruncateRunesHelpers(t *testing.T) {
	if truncateRunes("abcdef", 4) != "abc…" {
		t.Fatalf("got %q", truncateRunes("abcdef", 4))
	}
	if truncateRunesKeepTail("abcdef", 4) != "…def" {
		t.Fatalf("got %q", truncateRunesKeepTail("abcdef", 4))
	}
}
