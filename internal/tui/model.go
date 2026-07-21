package tui

import (
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/nanami/antisthenes/config"
	"github.com/nanami/antisthenes/internal/agent"
	"github.com/nanami/antisthenes/internal/memory"
	openai "github.com/sashabaranov/go-openai"
)

type Model struct {
	loop               *agent.Loop
	store              *memory.Store
	windows            [maxChatWindows]ChatWindow
	activeWindow       int
	textInput          textarea.Model
	viewport           viewport.Model
	width              int
	height             int
	ready              bool
	thinking           bool
	thinkingWindow     int
	spinnerFrame       int
	lastError          string
	cfg                config.Config
	showThinking       bool
	confirmCommand     string
	pendingDumpSummary bool
	pendingDumpWindow  int
	// Iterative /iterative job state lives per ChatWindow (one flow per window).
	approval     *approvalUI
	gatewayReply GatewayReplyFunc
	notify       NotifyFunc
	// Phase 3: chat-area tmux pane (above thinking/status; DESIGN.md horizontal split)
	tmuxEnabled     bool
	tmuxHost        string // empty = localhost
	tmuxSession     string
	tmuxPaneHeight  int
	tmuxContent     string
	tmuxLastErr     string
	tmuxViewport    viewport.Model
	tmuxCaptureBusy bool // one in-flight capture at a time
	tmuxCaptureSeq  int  // drop stale async results

	// Mouse mode (default on): wheel scrolls viewport; left-drag selects and copies.
	// /mouse off disables app mouse tracking so the terminal can native-select.
	mouseEnabled bool
	selSelecting bool
	selHasRange  bool
	selAnchor    selPos
	selEnd       selPos
}

// NotifyFunc posts messages into the running Program (typically p.Send).
type NotifyFunc func(tea.Msg)

type responseMsg struct {
	windowIndex int
	messages    []openai.ChatCompletionMessage
	err         error
}

// iterativeResultMsg is delivered when an async /iterative worker finishes.
// kind is perResultKindPlan (supervised gate) or perResultKindFull (terminal).
type iterativeResultMsg struct {
	win    int
	gen    int
	result string
	kind   string
}

// iterativeLogTickMsg schedules a non-blocking read of one window's work log.
type iterativeLogTickMsg struct {
	win int
	gen int
}

// iterativeLogProgressMsg carries newly appended work-log bytes into the TUI.
type iterativeLogProgressMsg struct {
	win       int
	gen       int
	chunk     string
	newOffset int64
}

type spinnerTickMsg struct{}

type CronResultMsg struct {
	Text string
}

type GatewayMsg struct {
	Text string
}

// NewModel builds the TUI with window 1 as the primary session. When telegramEnabled,
// window 2 is reserved for the configured instant messenger.
func NewModel(loop *agent.Loop, store *memory.Store, sessionID string, telegramEnabled bool) Model {
	cfg := config.Load()
	// Note: edit height applied in view/update per Phase 1. Slash hints when input
	// starts with `/`; Tab completes slash commands (handleSlashTabComplete).
	// textinput SetSuggestions was removed with the textarea migration.

	m := Model{
		loop:         loop,
		store:        store,
		textInput:    newTextInput(),
		cfg:          cfg,
		showThinking: cfg.ShowThinking,
		approval:     newApprovalUI(),
		mouseEnabled: true,
	}
	m.windows[0] = ChatWindow{
		Label:     "Chat",
		SessionID: sessionID,
	}
	if telegramEnabled && store != nil {
		if tgSid, err := store.CreateSession(); err == nil {
			m.windows[telegramWindowIndex] = ChatWindow{
				Label:     "Telegram",
				SessionID: tgSid,
			}
		}
	}
	m.loadWindowFromStore(0)
	if m.windows[telegramWindowIndex].SessionID != "" {
		m.loadWindowFromStore(telegramWindowIndex)
	}
	m.seedInputHistoryFromDisk()
	return m
}

// SetGatewayReply registers the callback used to send assistant replies to Telegram (window 2).
func (m *Model) SetGatewayReply(fn GatewayReplyFunc) {
	m.gatewayReply = fn
}

// SetNotify registers the callback used to deliver async tea.Msg values (e.g. agent responses).
func (m *Model) SetNotify(fn NotifyFunc) {
	m.notify = fn
}

func (m *Model) Init() tea.Cmd {
	return nil // textarea manages cursor; focus done in NewModel
}

// View in view.go; renderChat/wrap in render_chat.go; styles in styles.go; Update split across update_*.go.
