package tui

const approvalModalHint = "y — approve once    a — always (skip future prompts)    n — deny"

func approvalModalBody(tool, command string) string {
	if tool == "" {
		return command
	}
	return tool + ": " + command
}
