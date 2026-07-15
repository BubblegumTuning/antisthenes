package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func parseCopyFile(path string) []openai.ChatCompletionMessage {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var msgs []openai.ChatCompletionMessage
	for _, block := range strings.Split(string(data), "\n\n") {
		block = strings.TrimSpace(block)
		switch {
		case strings.HasPrefix(block, "You: "):
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    "user",
				Content: strings.TrimPrefix(block, "You: "),
			})
		case strings.HasPrefix(block, "Antisthenes: "):
			msgs = append(msgs, openai.ChatCompletionMessage{
				Role:    "assistant",
				Content: strings.TrimPrefix(block, "Antisthenes: "),
			})
		}
	}
	return msgs
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		if inEsc {
			if r == 'm' {
				inEsc = false
			}
			continue
		}
		if r == '\x1b' {
			inEsc = true
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func countNonEmptyLines(s string) int {
	n := 0
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(stripANSI(l)) != "" {
			n++
		}
	}
	return n
}

func countDuplicateContentLines(lines []string) map[string]int {
	seen := map[string]int{}
	for _, l := range lines {
		t := strings.TrimSpace(stripANSI(l))
		if t == "" {
			continue
		}
		seen[t]++
	}
	dups := map[string]int{}
	for k, v := range seen {
		if v > 1 {
			dups[k] = v
		}
	}
	return dups
}

func TestRenderChat_LineCountMatchesPlain_RealSession(t *testing.T) {
	msgs := parseCopyFile("/tmp/antisthenes-copy-20260709-191959.txt")
	if len(msgs) == 0 {
		t.Skip("copy fixture not available")
	}

	m := Model{width: 80, cfg: config.Config{AgentName: "Antisthenes"}}
	m.viewport = viewport.New(80, 24)
	m.windows[0].Messages = msgs

	plainLines := countNonEmptyLines(m.plainChatText())
	renderLines := countNonEmptyLines(m.renderChat())
	if renderLines > plainLines*2 {
		t.Fatalf("render has %d non-empty lines vs plain %d (too many)", renderLines, plainLines)
	}
}

func TestRenderChat_ViewportScroll_NoDuplicateLines(t *testing.T) {
	msgs := parseCopyFile("/tmp/antisthenes-copy-20260709-191959.txt")
	if len(msgs) == 0 {
		t.Skip("copy fixture not available")
	}
	m := Model{width: 80, cfg: config.Config{AgentName: "Antisthenes"}}
	m.windows[0].Messages = msgs
	rendered := m.renderChat()

	vp := viewport.New(80, 20)
	vp.SetContent(rendered)
	for y := 0; ; y++ {
		vp.SetYOffset(y)
		if dups := countDuplicateContentLines(strings.Split(vp.View(), "\n")); len(dups) > 0 {
			for line, n := range dups {
				t.Errorf("scroll y=%d duplicate x%d: %q", y, n, line[:min(60, len(line))])
			}
		}
		if vp.AtBottom() {
			break
		}
	}
}

func TestWrap_NoTrailingSpacePadding(t *testing.T) {
	text := "Hello world this is a longer test line"
	out := wrap(text, 20)
	for _, l := range strings.Split(out, "\n") {
		if strings.HasSuffix(l, "  ") {
			t.Errorf("wrap pads trailing spaces: %q", l)
		}
	}
}

func TestRenderChat_MarkdownMultiline_NoExtraLines(t *testing.T) {
	content := "Tampere is a vibrant city.\n\n## Overview\n- **Population:** ~250,000\n- **Region:** Pirkanmaa"
	m := Model{width: 80, cfg: config.Config{AgentName: "Antisthenes"}}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: content},
	}

	plainLines := countNonEmptyLines(m.plainMessageBlock(m.windows[0].Messages[0]))
	renderLines := countNonEmptyLines(m.renderChat())
	if renderLines > plainLines+2 {
		t.Fatalf("render has %d non-empty lines, plain has %d", renderLines, plainLines)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
