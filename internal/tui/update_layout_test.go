package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
)

func TestTruncateStatusPair_RightYieldsFirst(t *testing.T) {
	left := "model | 12% 2k/160k tokens | 14:00:00"
	right := "Cron: wrote summary to dump-abc.md"
	width := 40

	gotLeft, gotRight := truncateStatusPair(width, left, right)
	if gotLeft != left {
		t.Errorf("left changed unexpectedly: %q", gotLeft)
	}
	if gotRight == right {
		t.Error("right should be truncated on narrow width")
	}
	if len(gotRight) >= len(right) {
		t.Errorf("right not shortened: %q", gotRight)
	}
}

func TestTruncateStatusPair_TruncatesLeftWhenNeeded(t *testing.T) {
	left := strings.Repeat("M", 50)
	right := "note"
	gotLeft, gotRight := truncateStatusPair(20, left, right)
	if gotLeft == left {
		t.Error("left should truncate when both sides overflow")
	}
	if gotRight != "" {
		t.Errorf("right should yield entirely when left consumes width: %q", gotRight)
	}
}

func TestClampEditHeight_SmallTerminal(t *testing.T) {
	if got := clampEditHeight(5, 12); got != 1 {
		t.Errorf("clampEditHeight(5,12) = %d, want 1", got)
	}
	if got := clampEditHeight(3, 40); got != 3 {
		t.Errorf("clampEditHeight(3,40) = %d, want 3", got)
	}
}

func TestHandleWindowSize_SetsInputHeight(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		cfg:       config.Config{EditHeight: 3, AutoScroll: true},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	if mp.textInput.Height() != 3 {
		t.Errorf("textarea height = %d, want 3", mp.textInput.Height())
	}
	if !mp.ready {
		t.Error("expected ready after first resize")
	}
}

func TestHandleWindowSize_TinyTerminal(t *testing.T) {
	m := Model{
		textInput: textarea.New(),
		cfg:       config.Config{EditHeight: 5},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 30, Height: 14})
	if mp.viewport.Height < 0 {
		t.Errorf("viewport invalid: %d", mp.viewport.Height)
	}
	if mp.textInput.Height() < 1 {
		t.Error("edit box should keep at least one line")
	}
	if got := viewDisplayLines(mp.View()); got != mp.height {
		t.Errorf("view display lines=%d want terminal height=%d", got, mp.height)
	}
}
