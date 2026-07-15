package agent

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/nanami/antisthenes/internal/agent/installable"
	"github.com/nanami/antisthenes/internal/agent/prefcli"
)

func registerInstallTools(r *ToolRegistry) {
	r.Register("tool_status", func(args map[string]any) (string, error) {
		return executeToolStatus(args), nil
	})

	r.Register("install_tool", func(args map[string]any) (string, error) {
		return executeInstallTool(r, "install_tool", args)
	})
}

func executeToolStatus(args map[string]any) string {
	d := prefcli.DetectDistro()
	filter := "all"
	if raw, ok := args["tool"].(string); ok {
		name := strings.TrimSpace(raw)
		if name == "" || name == "all" {
			filter = "all"
		} else if id, ok := installable.ResolveID(name); ok && id != "all_missing" && id != "prefcli_missing" {
			filter = id
		} else {
			return installable.FormatStatus(d, name)
		}
	}
	return installable.FormatStatus(d, filter)
}

func executeInstallTool(r *ToolRegistry, toolName string, args map[string]any) (string, error) {
	requested, err := parseInstallToolRequest(args)
	if err != nil {
		return toolName + ": " + err.Error(), nil
	}

	d := prefcli.DetectDistro()

	pkgIDs, venvIDs, manual, already, unknown, err := installable.BuildInstallPlan(requested, d)
	if err != nil {
		return toolName + ": " + err.Error(), nil
	}

	if d == prefcli.DistroUnknown && (len(pkgIDs) > 0 || len(venvIDs) > 0) {
		return toolName + ": unsupported or unknown distro for automatic install", nil
	}

	var b strings.Builder
	if len(already) > 0 {
		b.WriteString("Already available: " + strings.Join(already, ", ") + "\n")
	}
	if len(unknown) > 0 {
		b.WriteString("Unknown tools: " + strings.Join(unknown, ", ") + "\n")
	}
	for _, m := range manual {
		b.WriteString(m + "\n")
	}

	if len(pkgIDs) == 0 && len(venvIDs) == 0 {
		if b.Len() == 0 {
			return toolName + ": nothing to install", nil
		}
		return strings.TrimRight(b.String(), "\n"), nil
	}

	installCmd, err := installable.InstallShellCommand(pkgIDs, venvIDs, d)
	if err != nil {
		return toolName + ": " + err.Error(), nil
	}

	targets := append(append([]string{}, pkgIDs...), venvIDs...)
	if ok, denied := r.requestInteractiveApproval(toolName, installCmd); !ok {
		if denied {
			return strings.TrimRight(b.String(), "\n") + "\n" + toolName + ": denied by user", nil
		}
		return strings.TrimRight(b.String(), "\n") + "\n" + toolName + ": approval required to install (" + strings.Join(targets, ", ") + "). Use approve_tool or approve via TUI popup.", nil
	}

	out, err := exec.Command("bash", "-c", installCmd).CombinedOutput()
	result := string(out)
	if err != nil {
		return strings.TrimRight(b.String(), "\n") + "\nError: " + err.Error() + "\n" + result, nil
	}

	b.WriteString(fmt.Sprintf("Install command completed for: %s\nOutput:\n%s", strings.Join(targets, ", "), result))

	var still []string
	for _, id := range targets {
		e, ok := installable.Get(id)
		if !ok || installable.Available(e) {
			continue
		}
		still = append(still, id)
	}
	if len(still) > 0 {
		b.WriteString("\nStill missing after install: " + strings.Join(still, ", "))
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func parseInstallToolRequest(args map[string]any) ([]string, error) {
	var requested []string
	if raw, ok := args["tool"].(string); ok && strings.TrimSpace(raw) != "" {
		requested = append(requested, strings.TrimSpace(raw))
	}
	switch t := args["tools"].(type) {
	case []any:
		for _, item := range t {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				requested = append(requested, strings.TrimSpace(s))
			}
		}
	case []string:
		for _, s := range t {
			if strings.TrimSpace(s) != "" {
				requested = append(requested, strings.TrimSpace(s))
			}
		}
	}
	if len(requested) == 0 {
		return nil, fmt.Errorf("tool or tools is required")
	}
	return requested, nil
}
