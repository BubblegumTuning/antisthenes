package tui

import (
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/nanami/antisthenes/internal/agent"
)

type approvalDecision struct {
	approved bool
	level    agent.ApprovalLevel
}

// approvalUI blocks tool execution until the user responds via the TUI modal.
type approvalUI struct {
	mu      sync.Mutex
	active  bool
	tool    string
	command string
	respCh  chan approvalDecision
}

func newApprovalUI() *approvalUI {
	return &approvalUI{}
}

func (a *approvalUI) begin(req agent.ApprovalRequest) approvalDecision {
	a.mu.Lock()
	ch := make(chan approvalDecision, 1)
	a.active = true
	a.tool = req.Tool
	a.command = req.Command
	a.respCh = ch
	a.mu.Unlock()

	d := <-ch

	a.mu.Lock()
	a.active = false
	a.tool = ""
	a.command = ""
	a.respCh = nil
	a.mu.Unlock()
	return d
}

func (a *approvalUI) respond(d approvalDecision) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.active || a.respCh == nil {
		return false
	}
	a.respCh <- d
	return true
}

func (a *approvalUI) isActive() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.active
}

func (a *approvalUI) currentTool() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.tool
}

func (a *approvalUI) currentCommand() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.command
}

func (m *Model) wireApprovalHandler() {
	if m.loop == nil || m.approval == nil {
		return
	}
	m.loop.SetApprovalHandler(func(req agent.ApprovalRequest) (bool, agent.ApprovalLevel) {
		if m.skipToolApprovalConfirm(req.Tool) {
			return true, agent.ApprovalOnce
		}
		d := m.approval.begin(req)
		return d.approved, d.level
	})
}

func (m Model) modalActive() bool {
	return (m.approval != nil && m.approval.isActive()) || m.confirmCommand != ""
}

func (m Model) renderModalOverlay(base string) string {
	var title, body, hint string
	if m.approval != nil && m.approval.isActive() {
		title = "Approval Required"
		body = approvalModalBody(m.approval.currentTool(), m.approval.currentCommand())
		hint = approvalModalHint
	} else if m.confirmCommand != "" {
		title = "Confirm Action"
		body = m.confirmCommand + " — proceed?"
		if isClearContextCommand(m.confirmCommand) {
			hint = "y — yes    a — always (skip future prompts)    n — no"
		} else {
			hint = "y — yes    n — no"
		}
	} else {
		return base
	}

	p := m.palette()
	modalW := m.width - 8
	if modalW < 30 {
		modalW = 30
	}
	box := p.modal.Width(modalW).Render(
		p.title.Render(title) + "\n\n" +
			body + "\n\n" +
			p.nudge.Render(hint),
	)
	if m.height < 10 || m.width < 24 {
		return base + "\n\n" + box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m *Model) handleModalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if !m.modalActive() {
		return nil, false
	}

	switch msg.Type {
	case tea.KeyEsc:
		if m.approval != nil && m.approval.isActive() {
			m.approval.respond(approvalDecision{approved: false, level: agent.ApprovalOnce})
		} else if m.confirmCommand != "" {
			m.confirmCommand = ""
		}
		m.textInput.Reset()
		return nil, true
	}

	var choice string
	switch msg.Type {
	case tea.KeyEnter:
		choice = strings.ToLower(strings.TrimSpace(m.textInput.Value()))
	case tea.KeyRunes:
		choice = strings.ToLower(string(msg.Runes))
	default:
		return nil, true // block other keys while modal is open
	}

	if choice == "" {
		return nil, true
	}

	if m.approval != nil && m.approval.isActive() {
		tool := m.approval.currentTool()
		switch choice {
		case "y", "yes":
			m.approval.respond(approvalDecision{approved: true, level: agent.ApprovalOnce})
		case "a", "always":
			m.persistToolApprovalWithoutConfirm(tool)
			m.approval.respond(approvalDecision{approved: true, level: agent.ApprovalOnce})
		case "n", "no":
			m.approval.respond(approvalDecision{approved: false, level: agent.ApprovalOnce})
		default:
			return nil, true
		}
		m.textInput.Reset()
		return nil, true
	}

	if m.confirmCommand != "" {
		switch choice {
		case "y", "yes":
			if isClearContextCommand(m.confirmCommand) {
				m.executeClearContext()
			}
			m.confirmCommand = ""
		case "a", "always":
			if isClearContextCommand(m.confirmCommand) {
				m.executeClearContext()
				m.persistClearWithoutConfirm()
			}
			m.confirmCommand = ""
		case "n", "no":
			m.confirmCommand = ""
		default:
			return nil, true
		}
		m.textInput.Reset()
		return nil, true
	}

	return nil, true
}
