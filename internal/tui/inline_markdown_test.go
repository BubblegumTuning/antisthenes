package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/nanami/antisthenes/config"
	openai "github.com/sashabaranov/go-openai"
)

func TestParseInlineMarkdown_Bold(t *testing.T) {
	segs := parseInlineMarkdown("hello **world**!")
	if len(segs) != 3 {
		t.Fatalf("segments=%d want 3: %+v", len(segs), segs)
	}
	if segs[1].text != "world" || segs[1].kind != mdBold {
		t.Fatalf("bold segment=%+v", segs[1])
	}
}

func TestRenderInlineMarkdown_BoldANSI(t *testing.T) {
	enableTestColors(t)
	out := renderInlineMarkdown("say **bold** now", 40)
	if !strings.Contains(out, "bold") {
		t.Fatalf("missing text: %q", out)
	}
	if !strings.Contains(out, "\x1b[1m") {
		t.Fatalf("expected bold ANSI in %q", out)
	}
}

func TestRenderInlineMarkdown_Code(t *testing.T) {
	out := renderInlineMarkdown("use `fmt.Println` here", 40)
	if !strings.Contains(out, "fmt.Println") {
		t.Fatalf("missing code text: %q", out)
	}
	if lipgloss.Width(out) == 0 {
		t.Fatal("empty render")
	}
}

func TestRenderInlineMarkdown_LinkStyled(t *testing.T) {
	enableTestColors(t)
	out := renderInlineMarkdown("see [docs](https://example.com)", 40)
	plain := stripANSI(out)
	if !strings.Contains(plain, "docs") {
		t.Fatalf("missing link label: %q", plain)
	}
	if !strings.Contains(out, "\x1b[4") {
		t.Fatalf("expected underline ANSI in %q", out)
	}
}

func TestRenderInlineMarkdown_UnclosedLiteral(t *testing.T) {
	out := renderInlineMarkdown("not **closed", 40)
	if !strings.Contains(out, "**closed") {
		t.Fatalf("unclosed delimiter should stay literal: %q", out)
	}
}

func TestRenderChat_AssistantMarkdown_UserLiteral(t *testing.T) {
	m := Model{
		width: 80,
		cfg:   config.Config{AgentName: "Bot", MarkdownEnabled: true},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "user", Content: "use **literal** stars"},
		{Role: "assistant", Content: "This is **important**"},
	}
	plain := stripANSI(m.renderChat())
	if !strings.Contains(plain, "You: use **literal** stars") {
		t.Fatalf("user message should keep markdown literal:\n%s", plain)
	}
	if !strings.Contains(plain, "Bot: This is important") {
		t.Fatalf("assistant should render bold text:\n%s", plain)
	}
	if strings.Contains(plain, "**important**") {
		t.Fatalf("assistant should not show raw delimiters:\n%s", plain)
	}
}

func TestRenderChat_MarkdownDisabled(t *testing.T) {
	m := Model{
		width: 80,
		cfg:   config.Config{AgentName: "Bot", MarkdownEnabled: false},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "This is **important**"},
	}
	out := stripANSI(m.renderChat())
	if !strings.Contains(out, "**important**") {
		t.Fatalf("disabled markdown should stay literal: %q", out)
	}
}