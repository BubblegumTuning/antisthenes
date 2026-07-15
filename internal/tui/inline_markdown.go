package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// MVP inline markdown styles (fixed palette). TODO: map to config theme — bold as
// brighter assistant color, code as tool-result tone (DESIGN-TUI.md).
var (
	inlineBold   = lipgloss.NewStyle().Bold(true)
	inlineItalic = lipgloss.NewStyle().Italic(true)
	inlineCode   = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
	inlineLink   = lipgloss.NewStyle().Underline(true).Foreground(lipgloss.Color("39"))
	inlineStrike = lipgloss.NewStyle().Strikethrough(true)
)

type mdStyleKind int

const (
	mdPlain mdStyleKind = iota
	mdBold
	mdItalic
	mdCode
	mdLink
	mdStrike
)

type mdSegment struct {
	text      string
	kind      mdStyleKind
	preStyled bool
}

func (m Model) markdownEnabled() bool {
	return m.cfg.MarkdownEnabled
}

// renderInlineMarkdown parses a subset of inline markdown and returns ANSI-wrapped,
// word-wrapped text. Block constructs (headings, lists, fences) are left literal.
func renderInlineMarkdown(text string, width int) string {
	if width <= 4 {
		return text
	}
	paragraphs := strings.Split(text, "\n")
	out := make([]string, len(paragraphs))
	for i, p := range paragraphs {
		if p == "" {
			out[i] = ""
			continue
		}
		out[i] = wrapStyledSegments(parseInlineMarkdown(p), width)
	}
	return strings.Join(out, "\n")
}

func parseInlineMarkdown(s string) []mdSegment {
	var segs []mdSegment
	i := 0
	for i < len(s) {
		if seg, n, ok := matchInlineMarkdown(s, i); ok {
			segs = append(segs, seg)
			i += n
			continue
		}
		j := i + 1
		for j < len(s) && !inlineMarkdownSpecial(s, j) {
			j++
		}
		segs = append(segs, mdSegment{text: s[i:j], kind: mdPlain})
		i = j
	}
	return mergePlainSegments(segs)
}

func inlineMarkdownSpecial(s string, i int) bool {
	if i >= len(s) {
		return false
	}
	switch s[i] {
	case '`', '[', '*', '_', '~':
		return true
	default:
		return false
	}
}

func matchInlineMarkdown(s string, i int) (mdSegment, int, bool) {
	if seg, n, ok := matchDelimited(s, i, "`", "`", mdCode); ok {
		return seg, n, true
	}
	if seg, n, ok := matchLink(s, i); ok {
		return seg, n, true
	}
	if seg, n, ok := matchDelimited(s, i, "**", "**", mdBold); ok {
		return seg, n, true
	}
	if seg, n, ok := matchDelimited(s, i, "__", "__", mdBold); ok {
		return seg, n, true
	}
	if seg, n, ok := matchDelimited(s, i, "~~", "~~", mdStrike); ok {
		return seg, n, true
	}
	if seg, n, ok := matchDelimited(s, i, "*", "*", mdItalic); ok {
		return seg, n, true
	}
	if seg, n, ok := matchDelimited(s, i, "_", "_", mdItalic); ok {
		return seg, n, true
	}
	return mdSegment{}, 0, false
}

func matchDelimited(s string, i int, open, close string, kind mdStyleKind) (mdSegment, int, bool) {
	if !strings.HasPrefix(s[i:], open) {
		return mdSegment{}, 0, false
	}
	if open == "*" && strings.HasPrefix(s[i:], "**") {
		return mdSegment{}, 0, false
	}
	if open == "_" && strings.HasPrefix(s[i:], "__") {
		return mdSegment{}, 0, false
	}
	rest := s[i+len(open):]
	end := strings.Index(rest, close)
	if end < 0 || end == 0 {
		return mdSegment{}, 0, false
	}
	inner := rest[:end]
	return mdSegment{text: inner, kind: kind}, len(open) + end + len(close), true
}

func matchLink(s string, i int) (mdSegment, int, bool) {
	if i >= len(s) || s[i] != '[' {
		return mdSegment{}, 0, false
	}
	closeLabel := strings.IndexByte(s[i+1:], ']')
	if closeLabel < 1 {
		return mdSegment{}, 0, false
	}
	labelEnd := i + 1 + closeLabel
	if labelEnd+1 >= len(s) || s[labelEnd+1] != '(' {
		return mdSegment{}, 0, false
	}
	closeURL := strings.IndexByte(s[labelEnd+2:], ')')
	if closeURL < 0 {
		return mdSegment{}, 0, false
	}
	label := s[i+1 : labelEnd]
	url := s[labelEnd+2 : labelEnd+2+closeURL]
	display := label
	if display == "" {
		display = url
	}
	styled := inlineLink.Render(display)
	if url != "" && url != label {
		styled += " (" + url + ")"
	}
	return mdSegment{text: styled, kind: mdPlain, preStyled: true}, labelEnd + 2 + closeURL + 1 - i, true
}

func renderMDKind(kind mdStyleKind, text string) string {
	switch kind {
	case mdBold:
		return inlineBold.Render(text)
	case mdItalic:
		return inlineItalic.Render(text)
	case mdCode:
		return inlineCode.Render(text)
	case mdStrike:
		return inlineStrike.Render(text)
	default:
		return text
	}
}

func mergePlainSegments(segs []mdSegment) []mdSegment {
	if len(segs) == 0 {
		return segs
	}
	out := []mdSegment{segs[0]}
	for _, seg := range segs[1:] {
		last := &out[len(out)-1]
		if last.kind == mdPlain && !last.preStyled && seg.kind == mdPlain && !seg.preStyled {
			last.text += seg.text
			continue
		}
		out = append(out, seg)
	}
	return out
}

func wrapStyledSegments(segs []mdSegment, width int) string {
	words := flattenSegmentWords(segs)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var cur strings.Builder
	curWidth := 0

	flush := func() {
		if cur.Len() > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
			curWidth = 0
		}
	}

	for _, w := range words {
		rendered := w.text
		if !w.preStyled && w.kind != mdPlain {
			rendered = renderMDKind(w.kind, w.text)
		}
		wWidth := lipgloss.Width(rendered)
		if wWidth > width {
			flush()
			lines = append(lines, hardWrapRendered(rendered, width)...)
			continue
		}
		if curWidth > 0 && curWidth+wWidth > width {
			flush()
		}
		cur.WriteString(rendered)
		curWidth += wWidth
	}
	flush()
	return strings.Join(lines, "\n")
}

type mdWord struct {
	text      string
	kind      mdStyleKind
	preStyled bool
}

func flattenSegmentWords(segs []mdSegment) []mdWord {
	var words []mdWord
	for _, seg := range segs {
		if seg.text == "" {
			continue
		}
		if seg.preStyled {
			words = append(words, mdWord{text: seg.text, preStyled: true})
			continue
		}
		start := 0
		for i := 0; i <= len(seg.text); i++ {
			if i == len(seg.text) || seg.text[i] == ' ' {
				if i > start {
					words = append(words, mdWord{text: seg.text[start:i], kind: seg.kind})
				}
				if i < len(seg.text) {
					words = append(words, mdWord{text: " ", kind: mdPlain})
				}
				start = i + 1
			}
		}
	}
	return words
}

func hardWrapRendered(rendered string, width int) []string {
	if width < 1 || rendered == "" {
		return []string{rendered}
	}
	var lines []string
	var cur strings.Builder
	curWidth := 0
	for len(rendered) > 0 {
		r, size := utf8.DecodeRuneInString(rendered)
		rendered = rendered[size:]
		ch := string(r)
		rw := lipgloss.Width(ch)
		if curWidth+rw > width && curWidth > 0 {
			lines = append(lines, cur.String())
			cur.Reset()
			curWidth = 0
		}
		cur.WriteString(ch)
		curWidth += rw
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}