package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if keyCmd, handled := m.handleKeyMsg(msg); handled {
			return m, keyCmd
		}
	case tea.MouseMsg:
		if mouseCmd, handled := m.handleMouseMsg(msg); handled {
			return m, mouseCmd
		}
	case responseMsg:
		if m.handleResponseMsg(msg) {
			return m, tea.ClearScreen
		}
		return m, nil
	case sessionTitleMsg:
		m.handleSessionTitleMsg(msg)
		return m, nil
	case iterativeResultMsg:
		if m.handleIterativeResult(msg) {
			return m, tea.ClearScreen
		}
		return m, nil
	case iterativeLogTickMsg:
		return m.handleIterativeLogTick(msg)
	case iterativeLogProgressMsg:
		m.handleIterativeLogProgress(msg)
		return m, nil
	case GatewayInboundMsg:
		return m, m.handleGatewayInbound(msg)
	case CronResultMsg:
		m.handleCronResult(msg)
		return m, nil
	case GatewayMsg:
		m.handleGatewayNotify(msg)
		return m, nil
	case spinnerTickMsg:
		return m.handleSpinnerTick()
	case tmuxTickMsg:
		return m.handleTmuxTick()
	case tmuxPaneMsg:
		m.handleTmuxPaneMsg(msg)
		return m, nil
	case tea.WindowSizeMsg:
		m.handleWindowSize(msg)
	}

	m.textInput, cmd = m.textInput.Update(msg)
	m.viewport, _ = m.viewport.Update(msg)
	if m.tmuxPaneVisible() {
		m.tmuxViewport, _ = m.tmuxViewport.Update(msg)
	}
	return m, cmd
}
