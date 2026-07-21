package tui

import (
	"strings"
	"testing"
)

func TestMatchingSlashCommands_BareSlash(t *testing.T) {
	matches := matchingSlashCommands("/")
	if len(matches) != len(slashCommands) {
		t.Fatalf("expected all %d commands, got %d", len(slashCommands), len(matches))
	}
}

func TestMatchingSlashCommands_Prefix(t *testing.T) {
	matches := matchingSlashCommands("/cle")
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = m.name
	}
	if !containsAll(names, "/clear", "/clear-history") {
		t.Fatalf("/cle should match clear commands, got %v", names)
	}
}

func TestMatchingSlashCommands_BuildWithArgs(t *testing.T) {
	matches := matchingSlashCommands("/build fix tests")
	if len(matches) != 1 || matches[0].name != "/build" {
		t.Fatalf("expected /build only, got %v", matches)
	}
}

func TestFormatSlashHint_BareSlash(t *testing.T) {
	out := formatSlashHint("/", 80)
	if !strings.Contains(out, "/exit") {
		t.Error("bare / hint should list /exit")
	}
	if !strings.Contains(out, "/help") {
		t.Error("bare / hint should mention /help")
	}
}

func TestFormatSlashHint_Filtered(t *testing.T) {
	out := formatSlashHint("/theme", 80)
	if !strings.Contains(out, "/theme") || !strings.Contains(out, "amber") {
		t.Errorf("filtered hint should describe /theme: %q", out)
	}
}

func TestFormatSlashHint_NoMatch(t *testing.T) {
	out := formatSlashHint("/zzzzz", 80)
	if !strings.Contains(out, "No matching") {
		t.Errorf("expected no-match message, got %q", out)
	}
}

func TestCompleteSlashInput(t *testing.T) {
	// Unique prefix
	got, ok := completeSlashInput("/hel")
	if !ok || got != "/help" {
		t.Fatalf("/hel -> /help, got %q ok=%v", got, ok)
	}
	// Args command gets trailing space
	got, ok = completeSlashInput("/bui")
	if !ok || got != "/build " {
		t.Fatalf("/bui -> /build , got %q ok=%v", got, ok)
	}
	// Shared prefix /cle -> LCP /clear
	got, ok = completeSlashInput("/cle")
	if !ok || got != "/clear" {
		t.Fatalf("/cle -> /clear LCP, got %q ok=%v", got, ok)
	}
	// Cycle from /clear to /clear-history
	got, ok = completeSlashInput("/clear")
	if !ok || got != "/clear-history" {
		t.Fatalf("/clear cycle -> /clear-history, got %q ok=%v", got, ok)
	}
	// With args: no change
	got, ok = completeSlashInput("/build fix tests")
	if ok || got != "/build fix tests" {
		t.Fatalf("args should not complete, got %q ok=%v", got, ok)
	}
	// Non-slash: no change, caller still consumes tab
	got, ok = completeSlashInput("hello")
	if ok {
		t.Fatalf("non-slash should not rewrite, got %q", got)
	}
}

func TestHandleSlashTabComplete(t *testing.T) {
	m := Model{textInput: newTextInput()}
	m.textInput.SetValue("/the")
	_, handled := m.handleSlashTabComplete()
	if !handled {
		t.Fatal("tab must be handled")
	}
	if m.textInput.Value() != "/theme " {
		t.Fatalf("value=%q", m.textInput.Value())
	}
}

func containsAll(haystack []string, needles ...string) bool {
	for _, n := range needles {
		found := false
		for _, h := range haystack {
			if h == n {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
