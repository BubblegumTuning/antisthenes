package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"

	"github.com/nanami/antisthenes/config"
)

type mdStyleKind int

const (
	mdPlain mdStyleKind = iota
	mdBold
	mdItalic
	mdCode
	mdLink
	mdStrike
	mdHeading
)

type mdSegment struct {
	text      string
	kind      mdStyleKind
	preStyled bool
}

func (m Model) markdownEnabled() bool {
	return m.cfg.MarkdownEnabled
}

// defaultMdStyles builds markdown styles from the default TUI palette (tests / helpers).
func defaultMdStyles() palette {
	return buildPalette(config.DefaultTUIColors())
}

// renderMarkdown renders assistant markdown: block constructs (headings, unordered
// lists, fenced code) plus the existing inline subset. Word-wraps to width.
// Styles come from the active palette (theme-aware). User/tool paths must not
// call this; /copy exports raw source separately.
func renderMarkdown(text string, width int, p palette) string {
	if width <= 4 {
		return text
	}
	lines := strings.Split(text, "\n")
	var out []string
	for i := 0; i < len(lines); {
		line := lines[i]

		if open, ok := parseFenceOpen(line); ok {
			var code []string
			i++
			for i < len(lines) {
				if isFenceClose(lines[i], open) {
					i++
					break
				}
				code = append(code, lines[i])
				i++
			}
			out = append(out, renderFencedCode(code, width, p)...)
			continue
		}

		if level, title, ok := parseATXHeading(line); ok {
			out = append(out, renderHeadingLines(level, title, width, p)...)
			i++
			continue
		}

		if bullet, rest, ok := parseUnorderedListItem(line); ok {
			out = append(out, renderListItemLines(bullet, rest, width, p)...)
			i++
			continue
		}

		if line == "" {
			out = append(out, "")
			i++
			continue
		}

		wrapped := wrapStyledSegments(parseInlineMarkdown(line), width, p)
		if wrapped == "" {
			out = append(out, "")
		} else {
			out = append(out, strings.Split(wrapped, "\n")...)
		}
		i++
	}
	return strings.Join(out, "\n")
}

// renderInlineMarkdown is the historical name; it now renders full (block+inline) markdown.
func renderInlineMarkdown(text string, width int) string {
	return renderMarkdown(text, width, defaultMdStyles())
}

// parseATXHeading matches "# " … "###### " headings (1–6 hashes + space).
func parseATXHeading(line string) (level int, title string, ok bool) {
	if line == "" || line[0] != '#' {
		return 0, "", false
	}
	n := 0
	for n < len(line) && n < 6 && line[n] == '#' {
		n++
	}
	if n == 0 || n > 6 {
		return 0, "", false
	}
	if n >= len(line) || line[n] != ' ' {
		return 0, "", false
	}
	title = strings.TrimSpace(line[n+1:])
	if title == "" {
		return 0, "", false
	}
	return n, title, true
}

// parseUnorderedListItem matches optional 0–3 leading spaces, then "- "/"* "/"+ ".
func parseUnorderedListItem(line string) (bullet, rest string, ok bool) {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	body := line[i:]
	for _, p := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(body, p) {
			return "• ", body[len(p):], true
		}
	}
	return "", "", false
}

func parseFenceOpen(line string) (marker string, ok bool) {
	// Allow up to 3 spaces of indent (CommonMark).
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	rest := line[i:]
	switch {
	case strings.HasPrefix(rest, "```"):
		return "```", true
	case strings.HasPrefix(rest, "~~~"):
		return "~~~", true
	default:
		return "", false
	}
}

func isFenceClose(line, open string) bool {
	i := 0
	for i < len(line) && i < 3 && line[i] == ' ' {
		i++
	}
	rest := strings.TrimRight(line[i:], " \t")
	if !strings.HasPrefix(rest, open) {
		return false
	}
	// Closing fence: only the fence chars (length ≥ open), no info string.
	for _, r := range rest[len(open):] {
		if r != rune(open[0]) {
			return false
		}
	}
	return true
}

func renderHeadingLines(level int, title string, width int, p palette) []string {
	_ = level
	segs := parseInlineMarkdown(title)
	// Emphasize plain runs as heading so they read stronger than body bold.
	for i := range segs {
		if segs[i].kind == mdPlain && !segs[i].preStyled {
			segs[i].kind = mdHeading
		}
	}
	wrapped := wrapStyledSegments(segs, width, p)
	if wrapped == "" {
		return []string{p.mdHeading.Render("")}
	}
	return strings.Split(wrapped, "\n")
}

func renderListItemLines(bullet, rest string, width int, p palette) []string {
	bulletW := lipgloss.Width(bullet)
	innerW := width - bulletW
	if innerW < 8 {
		innerW = width
		bulletW = 0
	}
	body := wrapStyledSegments(parseInlineMarkdown(rest), innerW, p)
	if body == "" {
		return []string{bullet}
	}
	lines := strings.Split(body, "\n")
	if bulletW == 0 {
		return lines
	}
	lines[0] = bullet + lines[0]
	indent := strings.Repeat(" ", bulletW)
	for i := 1; i < len(lines); i++ {
		lines[i] = indent + lines[i]
	}
	return lines
}

func renderFencedCode(code []string, width int, p palette) []string {
	if len(code) == 0 {
		// Empty fence: still show a styled blank line so structure is visible.
		return []string{p.mdCodeBlock.Render("")}
	}
	var out []string
	for _, raw := range code {
		// No inline markdown inside fences; plain wrap then style each visual line.
		plainWrap := wrap(raw, width)
		if plainWrap == "" {
			out = append(out, p.mdCodeBlock.Render(""))
			continue
		}
		for _, ln := range strings.Split(plainWrap, "\n") {
			out = append(out, p.mdCodeBlock.Render(ln))
		}
	}
	return out
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
	if url != "" && url != label {
		display += " (" + url + ")"
	}
	// Style applied at render time (theme-aware); not pre-styled.
	return mdSegment{text: display, kind: mdLink}, labelEnd + 2 + closeURL + 1 - i, true
}

func renderMDKind(kind mdStyleKind, text string, p palette) string {
	switch kind {
	case mdBold:
		return p.mdBold.Render(text)
	case mdHeading:
		return p.mdHeading.Render(text)
	case mdItalic:
		return p.mdItalic.Render(text)
	case mdCode:
		return p.mdCode.Render(text)
	case mdLink:
		return p.mdLink.Render(text)
	case mdStrike:
		return p.mdStrike.Render(text)
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

func wrapStyledSegments(segs []mdSegment, width int, p palette) string {
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
			rendered = renderMDKind(w.kind, w.text, p)
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
