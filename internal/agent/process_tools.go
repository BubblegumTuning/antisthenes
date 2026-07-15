package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func registerProcessTools(r *ToolRegistry) {
	r.Register("list_processes", func(args map[string]any) (string, error) {
		pattern, _ := args["pattern"].(string)
		pattern = strings.TrimSpace(pattern)
		user, _ := args["user"].(string)
		user = strings.TrimSpace(user)

		maxResults := 100
		switch v := args["max_results"].(type) {
		case float64:
			maxResults = int(v)
		case int:
			maxResults = v
		}
		if maxResults <= 0 {
			maxResults = 100
		}
		if maxResults > 500 {
			maxResults = 500
		}

		var out string
		var err error
		used := "ps"
		if pattern != "" {
			if _, lookErr := exec.LookPath("pgrep"); lookErr == nil {
				used = "pgrep"
				out, err = runCommandOutput("pgrep", "-af", pattern)
			} else {
				out, err = runCommandOutput("ps", "-eo", "pid,user,%cpu,%mem,etime,args", "--sort=-%cpu")
				if err == nil {
					out = filterProcessLines(out, pattern, user, maxResults)
					used = "ps+filter"
				}
			}
		} else {
			psArgs := []string{"-eo", "pid,user,%cpu,%mem,etime,args", "--sort=-%cpu"}
			out, err = runCommandOutput("ps", psArgs...)
			if err == nil && user != "" {
				out = filterProcessLines(out, "", user, maxResults)
				used = "ps+filter"
			}
		}
		if err != nil {
			return "", err
		}

		lines := strings.Split(strings.TrimSpace(out), "\n")
		if pattern != "" && used == "pgrep" {
			lines = truncateLines(lines, maxResults)
		} else if used == "ps" {
			lines = truncateLines(lines, maxResults)
		}
		if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
			return "list_processes: no matching processes", nil
		}
		result := strings.Join(lines, "\n")
		if len(lines) >= maxResults {
			result += fmt.Sprintf("\n... (truncated to %d lines)", maxResults)
		}
		return fmt.Sprintf("list_processes (via %s):\n%s", used, result), nil
	})

	r.Register("kill_process", func(args map[string]any) (string, error) {
		pid, err := parsePIDArg(args["pid"])
		if err != nil {
			return "kill_process: " + err.Error(), nil
		}
		if pid <= 1 {
			return "kill_process: refusing to kill system-critical pid", nil
		}
		if pid == os.Getpid() {
			return "kill_process: refusing to kill the agent process", nil
		}

		signalName, _ := args["signal"].(string)
		if strings.TrimSpace(signalName) == "" {
			signalName = "TERM"
		}
		sig, err := parseSignal(signalName)
		if err != nil {
			return "kill_process: " + err.Error(), nil
		}

		cmdStr := fmt.Sprintf("kill -%s %d", signalName, pid)
		if ok, denied := r.requestInteractiveApproval("kill_process", cmdStr); !ok {
			if denied {
				return "kill_process: denied by user", nil
			}
			return "kill_process: approval required. Use approve_tool or TUI popup.", nil
		}

		if err := syscall.Kill(pid, sig); err != nil {
			return "kill_process: " + err.Error(), nil
		}
		return fmt.Sprintf("kill_process: sent %s to pid %d", signalName, pid), nil
	})
}

func runCommandOutput(bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func filterProcessLines(out, pattern, user string, max int) string {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var filtered []string
	patLower := strings.ToLower(pattern)
	userLower := strings.ToLower(user)
	for _, line := range lines {
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if pattern != "" && !strings.Contains(lower, patLower) {
			continue
		}
		if user != "" {
			fields := strings.Fields(line)
			if len(fields) < 2 || strings.ToLower(fields[1]) != userLower {
				continue
			}
		}
		filtered = append(filtered, line)
		if len(filtered) >= max {
			break
		}
	}
	return strings.Join(filtered, "\n")
}

func truncateLines(lines []string, max int) []string {
	var out []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= max {
			break
		}
	}
	return out
}

func parsePIDArg(v any) (int, error) {
	switch n := v.(type) {
	case float64:
		return int(n), nil
	case int:
		return n, nil
	case string:
		return strconv.Atoi(strings.TrimSpace(n))
	default:
		return 0, fmt.Errorf("pid is required")
	}
}

func parseSignal(name string) (syscall.Signal, error) {
	name = strings.TrimSpace(strings.ToUpper(name))
	name = strings.TrimPrefix(name, "SIG")
	switch name {
	case "KILL", "9":
		return syscall.SIGKILL, nil
	case "TERM", "15":
		return syscall.SIGTERM, nil
	case "HUP", "1":
		return syscall.SIGHUP, nil
	case "INT", "2":
		return syscall.SIGINT, nil
	default:
		if n, err := strconv.Atoi(name); err == nil {
			return syscall.Signal(n), nil
		}
		return 0, fmt.Errorf("unknown signal %q", name)
	}
}
