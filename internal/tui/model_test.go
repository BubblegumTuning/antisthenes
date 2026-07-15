package tui

import (
	"testing"
)

func TestNewModel(t *testing.T) {
	m := NewModel(nil, nil, "test-sess", false)
	if m.windows[0].SessionID != "test-sess" {
		t.Errorf("sessionID = %q", m.windows[0].SessionID)
	}
	if m.textInput.Placeholder != "Type a message..." {
		t.Error("placeholder not set")
	}
}

func TestInit(t *testing.T) {
	m := &Model{}
	cmd := m.Init()
	if cmd != nil {
		t.Error("Init should return nil for textarea (focus done in NewModel)")
	}
}
