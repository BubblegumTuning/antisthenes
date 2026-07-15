package agent

import (
	"os"
	"testing"
)

func TestEvaluatePolicy(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"safe ls", "ls -la", false},
		{"dangerous rm", "rm -rf /", true},
		{"dd if", "dd if=/dev/zero of=/dev/sda", true},
		{"shutdown", "shutdown now", true},
		{"normal echo", "echo hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPolicy()
			got := p.NeedsApproval(tt.command)
			if got != tt.want {
				t.Errorf("NeedsApproval(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestPolicy_Approve(t *testing.T) {
	tests := []struct {
		name      string
		approve   string
		level     ApprovalLevel
		checkCmd  string
		wantNeeds bool
	}{
		{"once safe", "ls", ApprovalOnce, "ls -l", false},
		{"session dangerous", "rm -rf", ApprovalSession, "rm -rf /tmp", false},
		{"permanent prefix", "rm", ApprovalPermanent, "rm -rf foo", false},
		{"no approve dangerous", "", ApprovalOnce, "rm -rf /", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			orig, _ := os.Getwd()
			os.Chdir(tmp)
			defer os.Chdir(orig)
			defer os.Remove(".antisthenes_approvals")

			p := NewPolicy()
			if tt.approve != "" {
				p.Approve(tt.approve, tt.level)
			}
			got := p.NeedsApproval(tt.checkCmd)
			if got != tt.wantNeeds {
				t.Errorf("after Approve(%q, %v), NeedsApproval(%q) = %v, want %v",
					tt.approve, tt.level, tt.checkCmd, got, tt.wantNeeds)
			}
		})
	}
}

func TestPolicy_Approve_OnceIsOneShot(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)
	defer os.Remove(".antisthenes_approvals")

	p := NewPolicy()
	p.Approve("rm -rf", ApprovalOnce)

	if p.NeedsApproval("rm -rf /tmp") {
		t.Error("first use should not need approval")
	}
	if !p.NeedsApproval("rm -rf /tmp") {
		t.Error("second use after once should need approval again")
	}
}

func TestPolicy_ResetPermanent(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	p := NewPolicy()
	p.Approve("rm", ApprovalPermanent)

	if _, err := os.Stat(".antisthenes_approvals"); err != nil {
		t.Error("permanent approval should write .antisthenes_approvals")
	}
	if p.NeedsApproval("rm -rf /") {
		t.Error("should not need after permanent")
	}

	p.ResetPermanent()

	if p.NeedsApproval("rm -rf /") == false {
		t.Error("after ResetPermanent should need approval")
	}
	if _, err := os.Stat(".antisthenes_approvals"); err == nil {
		t.Error("ResetPermanent should remove .antisthenes_approvals")
	}
}

func TestPolicy_NeedsApproval_EdgeCases(t *testing.T) {
	p := NewPolicy()

	p.Approve("dd", ApprovalPermanent)
	if p.NeedsApproval("dd if=/dev/zero") {
		t.Error("permanent dd should bypass")
	}

	p.Approve("mkfs", ApprovalSession)
	if p.NeedsApproval("mkfs /dev/sda") {
		t.Error("session should bypass")
	}

	p.Approve("shutdown", ApprovalOnce)
	p.NeedsApproval("shutdown now")
	if !p.NeedsApproval("shutdown now") {
		t.Error("once should only bypass once")
	}
}
