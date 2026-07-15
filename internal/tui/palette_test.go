package tui

import (
	"testing"

	"github.com/nanami/antisthenes/config"
)

func TestBuildPalette_RendersNonEmpty(t *testing.T) {
	p := buildPalette(config.DefaultTUIColors())
	for name, out := range map[string]string{
		"user":      p.user.Render("You: "),
		"assistant": p.assistant.Render("Agent: "),
		"title":     p.title.Render("Title"),
		"status":    p.status.Render("status"),
	} {
		if out == "" {
			t.Errorf("%s style produced empty output", name)
		}
	}
}

func TestModelPalette_AppliesDefaults(t *testing.T) {
	m := Model{cfg: config.Config{Colors: config.TUIColors{User: "214"}}}
	m.cfg.Colors.ApplyDefaults()
	if m.cfg.Colors.Assistant != config.DefaultTUIColors().Assistant {
		t.Errorf("assistant default = %q", m.cfg.Colors.Assistant)
	}
	if m.palette().title.Render("x") == "" {
		t.Error("title style should render")
	}
}
