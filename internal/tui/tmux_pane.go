package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	openai "github.com/sashabaranov/go-openai"
)

const (
	defaultTmuxPaneHeight = 8
	tmuxTickInterval      = 1500 * time.Millisecond
	tmuxPaneTitleLines    = 1
)

// tmuxTickMsg triggers a background pane refresh while the pane is enabled.
type tmuxTickMsg struct{}

// tmuxPaneMsg carries async capture results into the TUI (keeps Update non-blocking).
type tmuxPaneMsg struct {
	content string
	err     string
	seq     int
}

func (m *Model) tmuxPaneVisible() bool {
	return m.tmuxEnabled && m.tmuxPaneHeight > 0
}

func (m *Model) tmuxChromeLines() int {
	if !m.tmuxPaneVisible() {
		return 0
	}
	return tmuxPaneTitleLines + m.tmuxPaneHeight
}

func (m *Model) ensureTmuxViewport() {
	if m.tmuxPaneHeight <= 0 {
		m.tmuxPaneHeight = defaultTmuxPaneHeight
	}
	w := m.width
	if w < 20 {
		w = 80
	}
	if m.tmuxViewport.Width == 0 {
		m.tmuxViewport = viewport.New(w, m.tmuxPaneHeight)
	} else {
		m.tmuxViewport.Width = w
		m.tmuxViewport.Height = m.tmuxPaneHeight
	}
	if m.tmuxContent != "" {
		m.tmuxViewport.SetContent(m.tmuxContent)
	}
}

func (m *Model) tmuxStatusSnippet() string {
	if !m.tmuxEnabled {
		return ""
	}
	host := m.tmuxHost
	if host == "" {
		host = "local"
	}
	sess := m.tmuxSession
	if sess == "" {
		sess = "antisthenes-persist"
	}
	return fmt.Sprintf("tmux:%s/%s", host, sess)
}

func (m *Model) enableTmuxPane(host, session string) tea.Cmd {
	m.tmuxEnabled = true
	if host != "" {
		m.tmuxHost = host
	}
	if session != "" {
		m.tmuxSession = session
	}
	if m.tmuxSession == "" {
		m.tmuxSession = "antisthenes-persist"
	}
	if m.tmuxPaneHeight <= 0 {
		m.tmuxPaneHeight = defaultTmuxPaneHeight
	}
	m.ensureTmuxViewport()
	m.resetViewportHeight()
	// Ensure session once on open; periodic ticks only capture.
	return tea.Batch(m.tmuxCaptureCmd(true), m.tmuxTickCmd())
}

func (m *Model) disableTmuxPane() {
	m.tmuxEnabled = false
	m.tmuxContent = ""
	m.tmuxLastErr = ""
	m.tmuxCaptureBusy = false
	m.resetViewportHeight()
}

func (m *Model) tmuxTickCmd() tea.Cmd {
	if !m.tmuxEnabled {
		return nil
	}
	return tea.Tick(tmuxTickInterval, func(time.Time) tea.Msg {
		return tmuxTickMsg{}
	})
}

// tmuxCaptureCmd runs capture off the UI goroutine via tea.Cmd.
// ensureSession=true only on first open / explicit host/session change.
func (m *Model) tmuxCaptureCmd(ensureSession bool) tea.Cmd {
	if !m.tmuxEnabled || m.loop == nil {
		return nil
	}
	if m.tmuxCaptureBusy {
		return nil
	}
	m.tmuxCaptureBusy = true
	m.tmuxCaptureSeq++
	seq := m.tmuxCaptureSeq
	host := m.tmuxHost
	session := m.tmuxSession
	paneH := m.tmuxPaneHeight
	if paneH <= 0 {
		paneH = defaultTmuxPaneHeight
	}
	reg := m.loop.Registry()
	return func() tea.Msg {
		if reg == nil {
			return tmuxPaneMsg{err: "no tool registry", seq: seq}
		}
		// Human-facing pane: raw pane text (no LLM header framing each tick).
		args := map[string]any{
			"lines":  float64(paneH + 40),
			"format": "raw",
		}
		if session != "" {
			args["session_name"] = session
		}
		if host != "" && host != "local" && host != "localhost" {
			args["host"] = host
		}
		if ensureSession {
			_, _ = reg.Call("tmux_attach_or_create", args)
		}
		out, err := reg.Call("tmux_capture", args)
		if err != nil {
			return tmuxPaneMsg{err: err.Error(), seq: seq}
		}
		return tmuxPaneMsg{content: out, seq: seq}
	}
}

func (m *Model) handleTmuxTick() (tea.Model, tea.Cmd) {
	if !m.tmuxEnabled {
		return m, nil
	}
	// Capture only when idle; always re-arm the tick so refresh resumes after busy windows.
	cap := m.tmuxCaptureCmd(false)
	return m, tea.Batch(cap, m.tmuxTickCmd())
}

func (m *Model) handleTmuxPaneMsg(msg tmuxPaneMsg) {
	if !m.tmuxEnabled {
		return
	}
	// Drop stale results from overlapping captures.
	if msg.seq != 0 && msg.seq != m.tmuxCaptureSeq {
		return
	}
	m.tmuxCaptureBusy = false
	var next string
	if msg.err != "" {
		m.tmuxLastErr = msg.err
		// Keep last good pane body on transient errors (title shows err).
		if strings.TrimSpace(m.tmuxContent) != "" && !strings.HasPrefix(m.tmuxContent, "[tmux] ") {
			return
		}
		next = "[tmux] " + msg.err
	} else {
		m.tmuxLastErr = ""
		next = msg.content
	}
	// Skip viewport mutation when text is unchanged — avoids scroll/flicker each tick.
	if next == m.tmuxContent {
		return
	}
	m.tmuxContent = next
	m.ensureTmuxViewport()
	m.tmuxViewport.SetContent(m.tmuxContent)
	m.tmuxViewport.GotoBottom()
}

func (m *Model) renderTmuxPane(p palette) string {
	if !m.tmuxPaneVisible() {
		return ""
	}
	m.ensureTmuxViewport()
	w := m.width
	if w < 20 {
		w = 80
	}
	host := m.tmuxHost
	if host == "" {
		host = "localhost"
	}
	sess := m.tmuxSession
	if sess == "" {
		sess = "antisthenes-persist"
	}
	title := fmt.Sprintf(" tmux │ %s @ %s │ /tmux off ", sess, host)
	if m.tmuxLastErr != "" {
		title = fmt.Sprintf(" tmux │ %s @ %s │ err │ /tmux off ", sess, host)
	}
	titleLine := renderFixedSlot(title, w, 1, p.dim)
	// Body: viewport already height-clamped.
	body := m.tmuxViewport.View()
	// Ensure exact pane height for layout stability.
	body = lipgloss.NewStyle().Width(w).Height(m.tmuxPaneHeight).MaxHeight(m.tmuxPaneHeight).Render(body)
	return titleLine + "\n" + body
}

// handleTmuxSlash processes /tmux and /tmux <subcommands>.
func (m *Model) handleTmuxSlash(input string) (tea.Cmd, bool) {
	parts := strings.Fields(input)
	if len(parts) == 0 || parts[0] != "/tmux" {
		return nil, false
	}
	assist := func(s string) {
		m.appendMessage(openai.ChatCompletionMessage{Role: "assistant", Content: s})
		m.syncViewport()
	}
	if len(parts) == 1 || parts[1] == "on" || parts[1] == "show" {
		cmd := m.enableTmuxPane(m.tmuxHost, m.tmuxSession)
		assist("Tmux pane enabled (session=" + m.tmuxSession + ", host=" + tmuxHostLabel(m.tmuxHost) + "). Use /tmux off, /tmux host <alias>, /tmux session <name>, /tmux refresh.")
		return cmd, true
	}
	switch parts[1] {
	case "off", "hide":
		m.disableTmuxPane()
		assist("Tmux pane disabled.")
		return nil, true
	case "refresh":
		if !m.tmuxEnabled {
			assist("Tmux pane is off. /tmux on to enable.")
			return nil, true
		}
		return m.tmuxCaptureCmd(false), true
	case "host":
		if len(parts) < 3 {
			assist("Usage: /tmux host <alias|local>")
			return nil, true
		}
		h := parts[2]
		if h == "local" || h == "localhost" {
			m.tmuxHost = ""
		} else {
			m.tmuxHost = h
		}
		var cmd tea.Cmd
		if m.tmuxEnabled {
			cmd = m.tmuxCaptureCmd(true)
		}
		assist("Tmux host set to " + tmuxHostLabel(m.tmuxHost) + ".")
		return cmd, true
	case "session":
		if len(parts) < 3 {
			assist("Usage: /tmux session <name>")
			return nil, true
		}
		m.tmuxSession = parts[2]
		var cmd tea.Cmd
		if m.tmuxEnabled {
			cmd = m.tmuxCaptureCmd(true)
		}
		assist("Tmux session set to " + m.tmuxSession + ".")
		return cmd, true
	case "status":
		state := "off"
		if m.tmuxEnabled {
			state = "on"
		}
		assist(fmt.Sprintf(
			"Tmux pane: %s | host=%s | session=%s | height=%d",
			state, tmuxHostLabel(m.tmuxHost), m.tmuxSession, m.tmuxPaneHeight,
		))
		return nil, true
	default:
		assist("Unknown /tmux subcommand. Try: on|off|refresh|host|session|status")
		return nil, true
	}
}

func tmuxHostLabel(host string) string {
	if host == "" {
		return "localhost"
	}
	return host
}
