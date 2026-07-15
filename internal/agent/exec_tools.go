package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// registerExecTools registers execution-related tools (bash and run_command with policy/safety).
func registerExecTools(r *ToolRegistry) {
	r.Register("bash", func(args map[string]any) (string, error) {
		cmdStr, ok := args["command"].(string)
		if !ok || cmdStr == "" {
			return "bash: command is required", nil
		}

		if ok, denied := r.resolveApproval("bash", cmdStr); !ok {
			if denied {
				return "bash: command denied by user", nil
			}
			return "bash: Approval required for this command. Use approve_tool or run with --approve flag.", nil
		}

		if strings.Contains(cmdStr, "rm -rf /") || strings.Contains(cmdStr, ":(){ :|:& };") {
			return "bash: command blocked for safety", nil
		}
		out, err := exec.Command("bash", "-c", cmdStr).CombinedOutput()
		if err != nil {
			return string(out) + "\nError: " + err.Error(), nil
		}
		return string(out), nil
	})

	r.Register("run_command", func(args map[string]any) (string, error) {
		cmdStr, ok := args["command"].(string)
		if !ok || strings.TrimSpace(cmdStr) == "" {
			return "run_command: command is required", nil
		}
		cmdStr = strings.TrimSpace(cmdStr)

		if ok, denied := r.resolveApproval("run_command", cmdStr); !ok {
			if denied {
				return "run_command: command denied by user", nil
			}
			return "run_command: approval required. Use approve_tool or TUI popup.", nil
		}

		if strings.Contains(cmdStr, "rm -rf /") || strings.Contains(cmdStr, ":(){ :|:& };") {
			return "run_command: command blocked for safety", nil
		}

		cwd, _ := args["cwd"].(string)
		cwd = strings.TrimSpace(cwd)
		if cwd != "" {
			if err := validateRelativePath(cwd); err != nil {
				return "run_command: cwd: " + err.Error(), nil
			}
		}

		background, _ := args["background"].(bool)
		if background {
			cmd := exec.Command("bash", "-c", cmdStr)
			applyRunCommandOpts(cmd, cwd, args)
			id, pid, err := r.jobs.start(cmd, cmdStr, cwd)
			if err != nil {
				return "run_command: " + err.Error(), nil
			}
			return fmt.Sprintf("run_command: started background job %d (pid %d). Use wait_job to collect output.", id, pid), nil
		}

		timeoutSec := parseTimeoutArg(args["timeout"])
		ctx := context.Background()
		var cancel context.CancelFunc
		if timeoutSec > 0 {
			ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
			defer cancel()
		}

		cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
		applyRunCommandOpts(cmd, cwd, args)

		out, err := cmd.CombinedOutput()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return string(out) + fmt.Sprintf("\nError: timed out after %ds", timeoutSec), nil
			}
			return string(out) + "\nError: " + err.Error(), nil
		}
		return string(out), nil
	})

	r.Register("wait_job", func(args map[string]any) (string, error) {
		id, err := parseJobIDArg(args["job_id"])
		if err != nil {
			return "wait_job: " + err.Error(), nil
		}
		timeoutSec := parseTimeoutArg(args["timeout"])
		return r.jobs.wait(id, timeoutSec)
	})

	r.Register("list_background_jobs", func(args map[string]any) (string, error) {
		return r.jobs.list(), nil
	})
}

func applyRunCommandOpts(cmd *exec.Cmd, cwd string, args map[string]any) {
	if cwd != "" {
		cmd.Dir = cwd
	}
	if envMap, ok := args["env"].(map[string]any); ok && len(envMap) > 0 {
		cmd.Env = mergeEnv(os.Environ(), envMap)
	}
}

func parseTimeoutArg(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func parseJobIDArg(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case string:
		var id int
		_, err := fmt.Sscanf(strings.TrimSpace(n), "%d", &id)
		if err != nil {
			return 0, fmt.Errorf("invalid job_id")
		}
		return id, nil
	default:
		return 0, fmt.Errorf("job_id is required")
	}
}

func mergeEnv(base []string, overrides map[string]any) []string {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, entry := range base {
		if k, v, ok := strings.Cut(entry, "="); ok {
			merged[k] = v
		}
	}
	for k, v := range overrides {
		merged[k] = fmt.Sprint(v)
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}
