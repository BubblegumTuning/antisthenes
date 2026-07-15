package agent

import (
	"os/exec"
)

// registerOtherTools registers goban plus search/patch and ansible tools (split into dedicated files).
func registerOtherTools(r *ToolRegistry) {
	registerGobanTools(r)
	registerSearchPatchTools(r)
	registerAnsibleTools(r)
}

func registerGobanTools(r *ToolRegistry) {
	r.Register("goban_create_ticket", func(args map[string]any) (string, error) {
		title, ok := args["title"].(string)
		if !ok || title == "" {
			return "goban_create_ticket: title is required", nil
		}

		desc, _ := args["description"].(string)
		column, _ := args["column"].(string)
		if column == "" {
			column = "todo"
		}
		cmd := exec.Command("goban-cli", "create", "-t", title, "-d", desc, "-c", column)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return string(out) + "\nError: " + err.Error(), nil
		}
		return string(out), nil
	})
}
