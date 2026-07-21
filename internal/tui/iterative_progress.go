package tui

import (
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	openai "github.com/sashabaranov/go-openai"
)

const (
	iterativeLogTickInterval = 1500 * time.Millisecond
	// Cap chat dump size per tick so a huge log write does not flood the viewport.
	iterativeLogChatMaxRunes = 2000
	iterativeSnippetMaxRunes = 72
)

// iterativeLogTickCmd schedules the next work-log poll for one job (win+gen).
func (m *Model) iterativeLogTickCmd(winIdx, gen int) tea.Cmd {
	if winIdx < 0 || winIdx >= maxChatWindows {
		return nil
	}
	w := &m.windows[winIdx]
	if w.iterState != IterExecuting || w.iterGen != gen {
		return nil
	}
	return tea.Tick(iterativeLogTickInterval, func(time.Time) tea.Msg {
		return iterativeLogTickMsg{win: winIdx, gen: gen}
	})
}

// iterativeLogReadCmd reads new bytes from the work log off the UI goroutine.
func (m *Model) iterativeLogReadCmd(winIdx, gen int) tea.Cmd {
	if winIdx < 0 || winIdx >= maxChatWindows {
		return nil
	}
	w := &m.windows[winIdx]
	if w.iterState != IterExecuting || w.iterGen != gen || w.iterCtx.LogFile == "" {
		return nil
	}
	path := w.iterCtx.LogFile
	offset := w.iterLogOffset
	return func() tea.Msg {
		chunk, newOff, err := readFileFromOffset(path, offset)
		if err != nil {
			// Missing/unreadable log is not fatal; keep offset and try again next tick.
			return iterativeLogProgressMsg{win: winIdx, gen: gen, newOffset: offset}
		}
		return iterativeLogProgressMsg{win: winIdx, gen: gen, chunk: string(chunk), newOffset: newOff}
	}
}

// handleIterativeLogTick kicks a background log read and reschedules the poll for that job.
func (m *Model) handleIterativeLogTick(msg iterativeLogTickMsg) (tea.Model, tea.Cmd) {
	if msg.win < 0 || msg.win >= maxChatWindows {
		return m, nil
	}
	w := &m.windows[msg.win]
	if w.iterState != IterExecuting || w.iterGen != msg.gen {
		return m, nil
	}
	return m, tea.Batch(
		m.iterativeLogReadCmd(msg.win, msg.gen),
		m.iterativeLogTickCmd(msg.win, msg.gen),
	)
}

// handleIterativeLogProgress applies new work-log text when still current for that window.
func (m *Model) handleIterativeLogProgress(msg iterativeLogProgressMsg) {
	if msg.win < 0 || msg.win >= maxChatWindows {
		return
	}
	w := &m.windows[msg.win]
	if msg.gen != w.iterGen || w.iterState != IterExecuting {
		return
	}
	w.iterLogOffset = msg.newOffset
	chunk := strings.TrimRight(msg.chunk, "\r\n")
	if chunk == "" {
		return
	}
	if snip := lastNonEmptyLine(chunk); snip != "" {
		w.iterProgressSnippet = truncateRunes(snip, iterativeSnippetMaxRunes)
	}
	// Surface progress in the owning window's chat.
	m.appendMessageTo(msg.win, openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: formatIterativeLogProgress(chunk),
	})
	m.persistWindowMessages(msg.win)
	if msg.win == m.activeWindow && m.ready {
		m.viewport.SetContent(m.renderChat())
		if m.cfg.AutoScroll {
			m.viewport.GotoBottom()
		}
	}
}

// readFileFromOffset returns bytes written after offset and the new end offset.
// If the file shrank (truncate), it restarts from 0.
func readFileFromOffset(path string, offset int64) ([]byte, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, offset, err
	}
	size := st.Size()
	if size < offset {
		offset = 0
	}
	if size == offset {
		return nil, offset, nil
	}
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, offset, err
	}
	return data, offset + int64(len(data)), nil
}

func formatIterativeLogProgress(chunk string) string {
	body := truncateRunesKeepTail(strings.TrimSpace(chunk), iterativeLogChatMaxRunes)
	return "[iterative log]\n" + body
}

func lastNonEmptyLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		t := strings.TrimSpace(lines[i])
		if t != "" {
			return t
		}
	}
	return ""
}

func truncateRunes(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max == 1 {
		return string(runes[0])
	}
	return string(runes[:max-1]) + "…"
}

func truncateRunesKeepTail(s string, max int) string {
	if max <= 0 || s == "" {
		return s
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	if max <= 2 {
		return string(runes[len(runes)-max:])
	}
	return "…" + string(runes[len(runes)-(max-1):])
}
