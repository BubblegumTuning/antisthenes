package agent

import (
	"os"
	"strings"
	"sync"
)

// ApprovalLevel defines the scope of an approval.
type ApprovalLevel int

const (
	ApprovalOnce ApprovalLevel = iota
	ApprovalSession
	ApprovalPermanent
)

// Policy manages tool execution approvals.
type Policy struct {
	mu                 sync.Mutex
	sessionApprovals   map[string]bool
	permanentApprovals map[string]bool
	onceApprovals      map[string]bool
}

// NewPolicy creates a new policy manager.
func NewPolicy() *Policy {
	return &Policy{
		sessionApprovals:   make(map[string]bool),
		permanentApprovals: make(map[string]bool),
		onceApprovals:      make(map[string]bool),
	}
}

// NeedsApproval checks if a command requires approval.
func (p *Policy) NeedsApproval(command string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for prefix := range p.permanentApprovals {
		if strings.HasPrefix(command, prefix) {
			return false
		}
	}
	for prefix := range p.sessionApprovals {
		if strings.HasPrefix(command, prefix) {
			return false
		}
	}
	for prefix := range p.onceApprovals {
		if strings.HasPrefix(command, prefix) {
			delete(p.onceApprovals, prefix)
			return false
		}
	}

	dangerous := []string{"rm -rf", "mkfs", ":(){", "dd if=", "shutdown", "reboot"}
	for _, d := range dangerous {
		if strings.Contains(command, d) {
			return true
		}
	}
	return false
}

// Approve grants approval at the specified level.
func (p *Policy) Approve(command string, level ApprovalLevel) {
	p.mu.Lock()
	defer p.mu.Unlock()

	switch level {
	case ApprovalOnce:
		p.onceApprovals[command] = true
	case ApprovalSession:
		p.sessionApprovals[command] = true
	case ApprovalPermanent:
		p.permanentApprovals[command] = true
		_ = os.WriteFile(".antisthenes_approvals", []byte(command+"\n"), 0o600)
	}
}

// ResetPermanent clears all permanent approvals.
func (p *Policy) ResetPermanent() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.permanentApprovals = make(map[string]bool)
	_ = os.Remove(".antisthenes_approvals")
}
