package tui

import (
	"fmt"

	"github.com/nanami/antisthenes/config"
)

// ApprovalTools lists tools that may show an interactive approval modal in the TUI.
var ApprovalTools = []string{
	"bash",
	"run_command",
	"create_dir",
	"delete_file",
	"move_file",
	"chmod",
	"git_add",
	"git_commit",
	"git_checkout",
	"git_branch",
	"install_modern_cli",
	"install_tool",
	"kill_process",
	"nmap_scan",
}

func (m Model) skipToolApprovalConfirm(tool string) bool {
	if tool == "" || m.cfg.ApprovalsWithoutConfirm == nil {
		return false
	}
	return m.cfg.ApprovalsWithoutConfirm[tool]
}

func (m *Model) persistToolApprovalWithoutConfirm(tool string) {
	if tool == "" {
		return
	}
	if m.cfg.ApprovalsWithoutConfirm == nil {
		m.cfg.ApprovalsWithoutConfirm = make(map[string]bool)
	}
	m.cfg.ApprovalsWithoutConfirm[tool] = true
	w := m.activeWin()
	if err := config.Save(m.cfg); err != nil {
		w.LastNotification = fmt.Sprintf("%s always approved (could not save config.json: %v)", tool, err)
		return
	}
	w.LastNotification = fmt.Sprintf("%s always approved — set approvals_without_confirm.%s to false in config.json to restore prompts", tool, tool)
}