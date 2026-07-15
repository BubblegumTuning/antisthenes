package config

import "testing"

func TestDefaultTUIColors_AmberGreenPalette(t *testing.T) {
	c := DefaultTUIColors()
	if c.User == "" || c.Assistant == "" || c.InputBorder == "" {
		t.Fatal("default palette missing primary keys")
	}
	if c.User != "214" {
		t.Errorf("expected amber user color 214, got %q", c.User)
	}
	if c.Assistant != "82" {
		t.Errorf("expected green assistant color 82, got %q", c.Assistant)
	}
}

func TestTUIColors_ApplyDefaults_PartialConfig(t *testing.T) {
	c := TUIColors{User: "99"}
	c.ApplyDefaults()
	if c.User != "99" {
		t.Error("explicit user color should be preserved")
	}
	if c.Assistant != DefaultTUIColors().Assistant {
		t.Error("empty assistant should get default")
	}
}

func TestTUIColors_ApplyDefaults_LegacyThinkingKey(t *testing.T) {
	c := TUIColors{Thinking: "123"}
	c.ApplyDefaults()
	if c.AssistantThinking != "123" {
		t.Errorf("legacy thinking key should map to assistant_thinking, got %q", c.AssistantThinking)
	}
}
