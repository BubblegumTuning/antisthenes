package agent

import "fmt"

// registerApprovalTools registers approval-related tools (approve_tool, reset_approvals).
func registerApprovalTools(r *ToolRegistry) {
	r.Register("approve_tool", func(args map[string]any) (string, error) {
		cmd, ok := args["command"].(string)
		if !ok || cmd == "" {
			return "approve_tool: command is required", nil
		}
		levelStr, _ := args["level"].(string)
		var level ApprovalLevel

		switch levelStr {
		case "permanent":
			level = ApprovalPermanent
		case "session":
			level = ApprovalSession
		default:
			level = ApprovalOnce
		}
		r.policy.Approve(cmd, level)
		return fmt.Sprintf("Approved '%s' (%s)", cmd, levelStr), nil
	})

	r.Register("reset_approvals", func(args map[string]any) (string, error) {
		r.policy.ResetPermanent()
		return "Permanent approvals have been reset.", nil
	})
}
