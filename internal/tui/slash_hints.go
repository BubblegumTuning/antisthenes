package tui

import (
	"fmt"
	"strings"
)

type slashCommand struct {
	name string
	hint string
}

// slashCommands is the canonical list for /help, padding hints, and prefix filtering.
var slashCommands = []slashCommand{
	{name: "/clear", hint: "clear context (y/n; a=always)"},
	{name: "/new", hint: "alias for /clear"},
	{name: "/compress", hint: "stub tools, keep last 20 msgs"},
	{name: "/dump-summary", hint: "write work summary, reset context"},
	{name: "/iterative", hint: "clarify goal, then autonomous build"},
	{name: "/build", hint: "autonomous build <task>"},
	{name: "/theme", hint: "green | amber phosphor palette"},
	{name: "/tools", hint: "list registered agent tools"},
	{name: "/tmux", hint: "pane on|off|host|session|refresh|status"},
	{name: "/clear-history", hint: "wipe input Up/Down history"},
	{name: "/copy", hint: "copy chat to clipboard (visible: /copy visible)"},
	{name: "/new_session", hint: "open window in slot 3–9"},
	{name: "/exit", hint: "quit (prints resume command)"},
	{name: "/help", hint: "full command reference"},
}

func matchingSlashCommands(input string) []slashCommand {
	input = strings.TrimSpace(input)
	if input == "" || input[0] != '/' {
		return nil
	}
	var out []slashCommand
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd.name, input) || strings.HasPrefix(input, cmd.name) {
			out = append(out, cmd)
		}
	}
	return out
}

// formatSlashHintSlot returns a single-line hint for the fixed padding slot above
// the edit box. Multi-line hints are truncated so toggling hints never grows View
// height (prevents ghost status bars in AltScreen scrollback).
func formatSlashHintSlot(input string, width int) string {
	full := strings.TrimSpace(formatSlashHint(input, width))
	if full == "" {
		return ""
	}
	line := full
	if idx := strings.IndexByte(full, '\n'); idx >= 0 {
		line = full[:idx]
	}
	return truncateDisplayWidth(line, width)
}

func formatSlashHint(input string, width int) string {
	if width < 20 {
		width = 76
	}
	matches := matchingSlashCommands(input)
	if len(matches) == 0 {
		return wrapSlashHintLine("No matching commands — try /help", width)
	}

	trimmed := strings.TrimSpace(input)
	// Bare "/" or ambiguous short prefix: compact name list.
	if trimmed == "/" || (len(matches) > 3 && !strings.Contains(trimmed, " ")) {
		names := make([]string, len(matches))
		for i, cmd := range matches {
			names[i] = cmd.name
		}
		line := "Slash: " + strings.Join(names, "  ") + "  — /help for details"
		return wrapSlashHintLine(line, width)
	}

	// Narrower match: show name + short hint per command.
	var parts []string
	for _, cmd := range matches {
		parts = append(parts, fmt.Sprintf("%s — %s", cmd.name, cmd.hint))
	}
	return wrapSlashHintLines(parts, width)
}

func wrapSlashHintLine(line string, width int) string {
	if width <= 0 {
		return line
	}
	return wrapSlashHintLines([]string{line}, width)
}

func wrapSlashHintLines(lines []string, width int) string {
	if width <= 0 {
		return strings.Join(lines, "\n")
	}
	var out []string
	for _, line := range lines {
		out = append(out, wrapSlashHintWords(line, width)...)
	}
	return strings.Join(out, "\n")
}

func wrapSlashHintWords(line string, width int) []string {
	words := strings.Fields(line)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	var b strings.Builder
	for _, w := range words {
		sep := 0
		if b.Len() > 0 {
			sep = 1
		}
		if b.Len()+sep+len(w) > width && b.Len() > 0 {
			lines = append(lines, strings.TrimSpace(b.String()))
			b.Reset()
			sep = 0
		}
		if sep == 1 {
			b.WriteByte(' ')
		}
		b.WriteString(w)
	}
	if b.Len() > 0 {
		lines = append(lines, strings.TrimSpace(b.String()))
	}
	return lines
}
