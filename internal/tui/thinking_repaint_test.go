package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestUpdate_ResponseMsg_RequestsClearScreenWhenThinking(t *testing.T) {
	m := Model{
		thinking:       true,
		thinkingWindow: 0,
		activeWindow:   0,
		ready:          true,
		width:          80,
		height:         24,
		viewport:       viewport.New(80, 8),
		textInput:      textarea.New(),
		cfg:            config.Config{AgentName: "T", EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	target := mp.viewport.Height
	mp.viewport.Height = 1 // simulate View() shrink while thinking

	msg := responseMsg{
		windowIndex: 0,
		messages:    []openai.ChatCompletionMessage{{Role: "assistant", Content: "reply"}},
	}
	updated, cmd := modelFromUpdate(mp, msg)
	if updated.thinking {
		t.Fatal("should stop thinking")
	}
	if cmd == nil {
		t.Fatal("expected ClearScreen cmd after visible thinking ends")
	}
	if updated.viewport.Height != target {
		t.Fatalf("viewport height=%d want restored target=%d", updated.viewport.Height, target)
	}
}

func TestBeginThinking_RequestsRepaintOnActiveWindow(t *testing.T) {
	m := Model{
		ready:     true,
		width:     80,
		height:    24,
		viewport:  viewport.New(80, 8),
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "T", EditHeight: 3},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	target := mp.viewport.Height
	mp.viewport.Height = 1

	if !mp.beginThinking(0) {
		t.Fatal("expected repaint when thinking starts on active window")
	}
	if !mp.thinking || mp.thinkingWindow != 0 {
		t.Fatal("thinking state not set")
	}
	if mp.viewport.Height != target {
		t.Fatalf("viewport height=%d want restored target=%d", mp.viewport.Height, target)
	}
}

func TestBeginThinking_NoRepaintOnOtherWindow(t *testing.T) {
	m := Model{activeWindow: 0, ready: true, width: 80, height: 24}
	mp := &m
	if mp.beginThinking(1) {
		t.Fatal("should not repaint when thinking starts on a background window")
	}
	if !mp.thinking || mp.thinkingWindow != 1 {
		t.Fatal("thinking state not set for background window")
	}
}

func TestSubmitUserMessage_RequestsRepaint(t *testing.T) {
	m := Model{
		ready:     true,
		width:     80,
		height:    24,
		viewport:  viewport.New(80, 8),
		textInput: textarea.New(),
		cfg:       config.Config{AgentName: "T"},
	}
	mp := &m
	mp.handleWindowSize(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !mp.submitUserMessage("hello") {
		t.Fatal("submit on active window should request repaint")
	}
}

func TestUpdate_ResponseMsg_NoClearScreenWhenThinkingOtherWindow(t *testing.T) {
	m := Model{
		thinking:       true,
		thinkingWindow: 1,
		activeWindow:   0,
		ready:          true,
		width:          80,
		height:         24,
		viewport:       viewport.New(80, 8),
		textInput:      textarea.New(),
		cfg:            config.Config{AgentName: "T"},
	}
	msg := responseMsg{
		windowIndex: 1,
		messages:    []openai.ChatCompletionMessage{{Role: "assistant", Content: "reply"}},
	}
	_, cmd := modelFromUpdate(&m, msg)
	if cmd != nil {
		t.Fatal("should not clear screen when thinking row was not on active window")
	}
}
