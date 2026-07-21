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
	{name: "/clear-history", hint: "wipe file-backed Up/Down history"},
	{name: "/copy", hint: "copy chat to clipboard (visible: /copy visible)"},
	{name: "/mouse", hint: "on|off — wheel+drag-copy vs native select"},
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

// slashCommandsTakingArgs get a trailing space after unique Tab completion.
var slashCommandsTakingArgs = map[string]bool{
	"/build": true,
	"/theme": true,
	"/tmux":  true,
	"/mouse": true,
	"/copy":  true,
}

// completeSlashInput applies Tab completion for slash commands on a single-line input.
// Returns the new value and whether it changed. Always consumes Tab in the caller
// (do not insert a literal tab into the edit box).
func completeSlashInput(input string) (string, bool) {
	if strings.ContainsAny(input, "\r\n") {
		return input, false
	}
	// Only complete the first token; leave args alone.
	token := input
	if i := strings.IndexByte(input, ' '); i >= 0 {
		if strings.TrimSpace(input[i:]) != "" {
			return input, false
		}
		token = input[:i]
	}
	if token == "" || token[0] != '/' {
		return input, false
	}

	var cands []string
	for _, cmd := range slashCommands {
		if strings.HasPrefix(cmd.name, token) {
			cands = append(cands, cmd.name)
		}
	}
	if len(cands) == 0 {
		return input, false
	}

	next := cands[0]
	if len(cands) > 1 {
		lcp := longestCommonPrefix(cands)
		if lcp != token {
			next = lcp
		} else {
			// Already at LCP (or exact member): cycle through candidates.
			idx := -1
			for i, c := range cands {
				if c == token {
					idx = i
					break
				}
			}
			next = cands[(idx+1)%len(cands)]
		}
	}

	out := next
	if slashCommandsTakingArgs[out] {
		out += " "
	}
	if out == input {
		return input, false
	}
	return out, true
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if prefix == "" {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
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
