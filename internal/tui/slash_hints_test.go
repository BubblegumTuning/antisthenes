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
