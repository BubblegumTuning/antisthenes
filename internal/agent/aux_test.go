package agent

import (
	"strings"
	"testing"

	"github.com/nanami/antisthenes/config"
)

func TestHeuristicSessionTitle(t *testing.T) {
	got := HeuristicSessionTitle("  fix the fts clear bug please\nmore")
	if !strings.Contains(strings.ToLower(got), "fix") {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("word ", 40)
	got = HeuristicSessionTitle(long)
	if len([]rune(got)) > 55 {
		t.Fatalf("too long: %q (%d runes)", got, len([]rune(got)))
	}
}

func TestSanitizeSessionTitle(t *testing.T) {
	got := SanitizeSessionTitle("Title: \"SQLite FTS cleanup\"\n")
	if got == "" || strings.Contains(got, "Title:") {
		t.Fatalf("got %q", got)
	}
}

func TestRegisterAuxExecutorsAndListTools(t *testing.T) {
	cfg := config.Config{
		AuxModels: []config.AuxModel{
			{Name: "title-mini", Model: "m", BaseURL: "http://127.0.0.1:9/v1", Roles: []string{"title"}},
		},
	}
	RegisterAuxExecutors(cfg)
	ex := GetExecutor("title-mini")
	if ex.Model != "m" {
		t.Fatalf("executor not registered: %+v", ex)
	}
	reg := NewToolRegistry()
	RegisterAuxTools(reg, cfg)
	out, err := reg.Call("list_aux_models", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "title-mini") {
		t.Fatalf("list output: %s", out)
	}
}

func TestGenerateSessionTitle_HeuristicOnly(t *testing.T) {
	// No aux models → heuristic path.
	title := GenerateSessionTitle(nil, config.Config{}, "rename demo database file")
	if title == "" {
		t.Fatal("empty title")
	}
}
