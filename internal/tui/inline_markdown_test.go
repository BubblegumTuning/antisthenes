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
	if !strings.Contains(out, "\x1b[1") {
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

func TestParseATXHeading(t *testing.T) {
	level, title, ok := parseATXHeading("## Hello **world**")
	if !ok || level != 2 || title != "Hello **world**" {
		t.Fatalf("got level=%d title=%q ok=%v", level, title, ok)
	}
	if _, _, ok := parseATXHeading("#nospace"); ok {
		t.Fatal("heading without space should fail")
	}
	if _, _, ok := parseATXHeading("####### seven"); ok {
		t.Fatal("7 hashes should fail")
	}
}

func TestParseUnorderedListItem(t *testing.T) {
	b, rest, ok := parseUnorderedListItem("- first item")
	if !ok || b != "• " || rest != "first item" {
		t.Fatalf("got bullet=%q rest=%q ok=%v", b, rest, ok)
	}
	if _, _, ok := parseUnorderedListItem("*italic alone*"); ok {
		t.Fatal("*italic* must not be a list item")
	}
	_, rest, ok = parseUnorderedListItem("  * indented")
	if !ok || rest != "indented" {
		t.Fatalf("indent list: rest=%q ok=%v", rest, ok)
	}
}

func TestRenderMarkdown_HeadingListFence(t *testing.T) {
	enableTestColors(t)
	p := defaultMdStyles()
	src := "# Title\n\n## Section\n\n- item **one**\n- item two\n\n```go\nfmt.Println(\"hi\")\n```\n\nafter"
	out := renderMarkdown(src, 60, p)
	plain := stripANSI(out)
	if strings.Contains(plain, "# Title") || strings.Contains(plain, "## Section") {
		t.Fatalf("heading markers should be stripped:\n%s", plain)
	}
	if !strings.Contains(plain, "Title") || !strings.Contains(plain, "Section") {
		t.Fatalf("heading text missing:\n%s", plain)
	}
	if !strings.Contains(plain, "• ") {
		t.Fatalf("list bullet missing:\n%s", plain)
	}
	if strings.Contains(plain, "**one**") {
		t.Fatalf("list should render inline bold:\n%s", plain)
	}
	if !strings.Contains(plain, `fmt.Println("hi")`) {
		t.Fatalf("fence body missing:\n%s", plain)
	}
	if strings.Contains(plain, "```") {
		t.Fatalf("fence markers should not appear:\n%s", plain)
	}
	if !strings.Contains(out, "\x1b[1") {
		t.Fatalf("expected bold ANSI for heading/list emphasis: %q", out)
	}
	// Fence must not parse inline markdown inside.
	fenceOnly := renderMarkdown("```\nuse **raw** stars\n```", 40, p)
	if !strings.Contains(stripANSI(fenceOnly), "**raw**") {
		t.Fatalf("code fence must keep literal markdown:\n%s", stripANSI(fenceOnly))
	}
}

func TestRenderMarkdown_TildeFence(t *testing.T) {
	out := stripANSI(renderMarkdown("~~~\nline\n~~~", 40, defaultMdStyles()))
	if !strings.Contains(out, "line") || strings.Contains(out, "~~~") {
		t.Fatalf("tilde fence: %q", out)
	}
}

func TestRenderMarkdown_ThemeAwareColors(t *testing.T) {
	enableTestColors(t)
	src := "**bold** and `code` and [docs](https://example.com)\n\n# Heading\n\n```\nfence\n```"
	green := buildPalette(config.GreenPhosphorColors())
	amber := buildPalette(config.AmberPhosphorColors())
	gOut := renderMarkdown(src, 80, green)
	aOut := renderMarkdown(src, 80, amber)
	if stripANSI(gOut) != stripANSI(aOut) {
		t.Fatalf("plain text should match across themes\ngreen=%q\namber=%q", stripANSI(gOut), stripANSI(aOut))
	}
	if gOut == aOut {
		t.Fatalf("styled output should differ between green and amber themes")
	}
	// Bold uses assistant token; green assistant=118, amber=214.
	if !strings.Contains(green.mdBold.Render("x"), "x") || !strings.Contains(amber.mdBold.Render("x"), "x") {
		t.Fatal("mdBold must render text")
	}
	if green.mdBold.GetForeground() == amber.mdBold.GetForeground() {
		t.Fatalf("bold FG should track assistant theme color")
	}
	if green.mdCode.GetForeground() == amber.mdCode.GetForeground() {
		t.Fatalf("code FG should track tool_result theme color")
	}
	if green.mdLink.GetForeground() == amber.mdLink.GetForeground() {
		t.Fatalf("link FG should track status theme color")
	}
	if green.mdHeading.GetForeground() == amber.mdHeading.GetForeground() {
		t.Fatalf("heading FG should track title theme color")
	}
	// Chat path uses model palette (not package defaults).
	m := Model{
		width: 80,
		cfg: config.Config{
			AgentName:       "Bot",
			MarkdownEnabled: true,
			Colors:          config.GreenPhosphorColors(),
		},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "see `code` and **bold**"},
	}
	chat := m.renderChat()
	if !strings.Contains(stripANSI(chat), "code") || !strings.Contains(stripANSI(chat), "bold") {
		t.Fatalf("chat markdown missing text: %q", stripANSI(chat))
	}
	if chat == stripANSI(chat) {
		t.Fatalf("expected ANSI styling in themed chat render")
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

func TestRenderChat_AssistantBlockMarkdown(t *testing.T) {
	m := Model{
		width: 80,
		cfg:   config.Config{AgentName: "Bot", MarkdownEnabled: true},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "## Plan\n\n- step one\n\n```\ncode_here\n```"},
	}
	plain := stripANSI(m.renderChat())
	if strings.Contains(plain, "## Plan") {
		t.Fatalf("heading hashes should not show:\n%s", plain)
	}
	if !strings.Contains(plain, "Plan") || !strings.Contains(plain, "• step one") {
		t.Fatalf("block render missing:\n%s", plain)
	}
	if !strings.Contains(plain, "code_here") || strings.Contains(plain, "```") {
		t.Fatalf("fence render:\n%s", plain)
	}
}

func TestRenderChat_MarkdownDisabled(t *testing.T) {
	m := Model{
		width: 80,
		cfg:   config.Config{AgentName: "Bot", MarkdownEnabled: false},
	}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "## Title\n\nThis is **important**"},
	}
	out := stripANSI(m.renderChat())
	if !strings.Contains(out, "**important**") || !strings.Contains(out, "## Title") {
		t.Fatalf("disabled markdown should stay literal: %q", out)
	}
}

func TestPlainChatText_KeepsRawMarkdown(t *testing.T) {
	// /copy uses plainChatText — must not run the renderer.
	m := Model{cfg: config.Config{AgentName: "Bot", MarkdownEnabled: true}}
	m.windows[0].Messages = []openai.ChatCompletionMessage{
		{Role: "assistant", Content: "## H\n- a\n```\nx\n```"},
	}
	plain := m.plainChatText()
	if !strings.Contains(plain, "## H") || !strings.Contains(plain, "```") {
		t.Fatalf("copy path must stay raw:\n%s", plain)
	}
}
