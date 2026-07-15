package agent

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultTmuxSession = "antisthenes-persist"

// tmuxSessionNameRE allows only safe tmux target / host-alias names.
var tmuxSessionNameRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// registerTmuxTools registers Phase 0–2 tmux tools (local + SSH hosts + capture formats).
func registerTmuxTools(r *ToolRegistry) {
	attachFn := func(args map[string]any) (string, error) {
		if err := requireTmuxLocalOrSSH(args); err != nil {
			return err.Error(), nil
		}
		host, err := hostFromArgs(args)
		if err != nil {
			return "tmux_attach: " + err.Error(), nil
		}
		session, err := sessionNameForHost(args, host)
		if err != nil {
			return "tmux_attach: " + err.Error(), nil
		}
		created, msg, err := ensureTmuxSession(host, session)
		if err != nil {
			return msg, nil
		}
		if created {
			return "tmux session " + session + " created" + hostSuffix(host) + " (interactive shell)", nil
		}
		return "tmux session " + session + " already exists" + hostSuffix(host), nil
	}

	// Phase 2 primary name; tmux_attach kept as alias.
	r.Register("tmux_attach_or_create", attachFn)
	r.Register("tmux_attach", attachFn)

	r.Register("tmux_send", func(args map[string]any) (string, error) {
		if err := requireTmuxLocalOrSSH(args); err != nil {
			return err.Error(), nil
		}
		host, err := hostFromArgs(args)
		if err != nil {
			return "tmux_send: " + err.Error(), nil
		}
		session, err := sessionNameForHost(args, host)
		if err != nil {
			return "tmux_send: " + err.Error(), nil
		}
		keys, ok := args["keys"].(string)
		if !ok || strings.TrimSpace(keys) == "" {
			return "tmux_send: keys required", nil
		}

		if ok, denied := r.resolveApproval("tmux_send", keys); !ok {
			if denied {
				return "tmux_send: command denied by user", nil
			}
			return "tmux_send: Approval required for this input. Use approve_tool or run with --approve flag.", nil
		}
		if strings.Contains(keys, "rm -rf /") || strings.Contains(keys, ":(){ :|:& };") {
			return "tmux_send: command blocked for safety", nil
		}

		// Phase 2: on-demand creation (one long-lived session per host/session name).
		created, msg, err := ensureTmuxSession(host, session)
		if err != nil {
			return "tmux_send: " + msg, nil
		}
		prefix := ""
		if created {
			prefix = "(created session) "
		}

		if out, err := runTmux(host, "send-keys", "-t", session, "-l", keys); err != nil {
			return out + "\nError: " + err.Error(), nil
		}
		if out, err := runTmux(host, "send-keys", "-t", session, "Enter"); err != nil {
			return out + "\nError: " + err.Error(), nil
		}
		return prefix + "sent to " + session + hostSuffix(host) + ": " + keys, nil
	})

	r.Register("tmux_capture", func(args map[string]any) (string, error) {
		if err := requireTmuxLocalOrSSH(args); err != nil {
			return err.Error(), nil
		}
		host, err := hostFromArgs(args)
		if err != nil {
			return "tmux_capture: " + err.Error(), nil
		}
		session, err := sessionNameForHost(args, host)
		if err != nil {
			return "tmux_capture: " + err.Error(), nil
		}
		lines := tmuxLinesFromArgs(args)
		format := tmuxCaptureFormat(args)

		// On-demand: empty pane if we just created; still valid for LLM.
		if _, _, err := ensureTmuxSession(host, session); err != nil {
			return "tmux_capture: session unavailable" + hostSuffix(host), nil
		}

		text, errMsg := captureTmuxPane(host, session, lines)
		if errMsg != "" {
			return errMsg, nil
		}
		if strings.TrimSpace(text) == "" {
			return formatTmuxCapture(format, session, host, lines, "(no output in pane)"), nil
		}
		return formatTmuxCapture(format, session, host, lines, text), nil
	})

	r.Register("tmux_list_sessions", func(args map[string]any) (string, error) {
		if err := requireTmuxLocalOrSSH(args); err != nil {
			return err.Error(), nil
		}
		host, err := hostFromArgs(args)
		if err != nil {
			return "tmux_list_sessions: " + err.Error(), nil
		}
		out, err := runTmux(host, "list-sessions")
		if err != nil {
			if strings.Contains(out, "no server running") || strings.Contains(out, "no sessions") ||
				strings.Contains(err.Error(), "no server") {
				return "no tmux sessions" + hostSuffix(host), nil
			}
			return out + "\nError: " + err.Error(), nil
		}
		if strings.TrimSpace(out) == "" {
			return "no tmux sessions" + hostSuffix(host), nil
		}
		return out, nil
	})

	r.Register("tmux_kill_session", func(args map[string]any) (string, error) {
		if err := requireTmuxLocalOrSSH(args); err != nil {
			return err.Error(), nil
		}
		host, err := hostFromArgs(args)
		if err != nil {
			return "tmux_kill_session: " + err.Error(), nil
		}
		session, err := sessionNameForHost(args, host)
		if err != nil {
			return "tmux_kill_session: " + err.Error(), nil
		}
		out, err := runTmux(host, "kill-session", "-t", session)
		if err != nil {
			return out + "\nError: " + err.Error(), nil
		}
		return "killed tmux session " + session + hostSuffix(host), nil
	})

	// Phase 1: host registration
	r.Register("tmux_register_host", func(args map[string]any) (string, error) {
		name, _ := args["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			return "tmux_register_host: name required (alias for host=)", nil
		}
		if !hostAliasREMatch(name) {
			return "tmux_register_host: invalid name (use letters, digits, _ and -)", nil
		}
		if name == "local" || name == "localhost" {
			return "tmux_register_host: name reserved for local execution", nil
		}
		hostAddr, _ := args["host"].(string)
		hostAddr = strings.TrimSpace(hostAddr)
		user, _ := args["user"].(string)
		user = strings.TrimSpace(user)
		keyRaw, _ := args["key_path"].(string)
		keyPath := expandHomePath(keyRaw)
		if hostAddr == "" || user == "" || keyPath == "" {
			return "tmux_register_host: host, user, and key_path are required", nil
		}
		if err := validateTmuxHostKey(keyPath); err != nil {
			return "tmux_register_host: " + err.Error(), nil
		}

		session := defaultTmuxSession
		if s, ok := args["session_name"].(string); ok && strings.TrimSpace(s) != "" {
			session = strings.TrimSpace(s)
			if !tmuxSessionNameRE.MatchString(session) {
				return "tmux_register_host: invalid session_name", nil
			}
		}
		port := 22
		switch v := args["port"].(type) {
		case float64:
			port = int(v)
		case int:
			port = v
		}
		if port <= 0 || port > 65535 {
			return "tmux_register_host: invalid port", nil
		}

		doValidate := true
		if v, ok := args["validate"].(bool); ok {
			doValidate = v
		}

		h := TmuxHost{
			Name:        name,
			Host:        hostAddr,
			User:        user,
			KeyPath:     keyPath,
			SessionName: session,
			Port:        port,
		}
		if doValidate {
			if err := validateTmuxHostSSH(h); err != nil {
				return "tmux_register_host: " + err.Error() + " (re-run with validate=false to store without SSH check)", nil
			}
		}
		if err := upsertTmuxHost(h); err != nil {
			return "tmux_register_host: save failed: " + err.Error(), nil
		}
		msg := fmt.Sprintf("registered host %s (%s@%s:%d) key=%s session=%s",
			h.Name, h.User, h.Host, port, h.KeyPath, h.SessionName)
		if !doValidate {
			msg += " (SSH check skipped)"
		} else {
			msg += " (SSH check ok)"
		}
		return msg, nil
	})

	r.Register("tmux_list_hosts", func(args map[string]any) (string, error) {
		reg, err := loadTmuxHosts()
		if err != nil {
			return "tmux_list_hosts: " + err.Error(), nil
		}
		return formatHostList(reg), nil
	})
}

// ensureTmuxSession creates a detached interactive session if missing.
// Returns created=true when a new session was started.
func ensureTmuxSession(host *TmuxHost, session string) (created bool, errMsg string, err error) {
	if _, e := runTmux(host, "has-session", "-t", session); e == nil {
		return false, "", nil
	}
	// Default interactive shell (login/profile). Avoid bash --norc here: on some
	// hosts send/capture against a norc pane yields empty/corrupt buffers.
	out, e := runTmux(host, "new-session", "-d", "-s", session)
	if e != nil && !strings.Contains(out, "duplicate session") {
		return false, out + "\nError: " + e.Error(), e
	}
	_, _ = runTmux(host, "set-option", "-t", session, "remain-on-exit", "on")
	if _, e := runTmux(host, "has-session", "-t", session); e != nil {
		return false, "failed to create persistent tmux session " + session + hostSuffix(host), e
	}
	// Settle so the shell prompt is ready for send-keys.
	time.Sleep(200 * time.Millisecond)
	return true, "", nil
}

func captureTmuxPane(host *TmuxHost, session string, lines int) (text string, errMsg string) {
	// Requires tmux 3.7+ (capture-pane -p is reliable; next-3.4 on el10 was not).
	out, err := runTmux(host, "capture-pane", "-p", "-t", session, "-S", "-"+strconv.Itoa(lines))
	if err != nil {
		if isTmuxUnreachable(out, err) {
			return "", "tmux session appears dead or unreachable"
		}
		return "", out + "\nError: " + err.Error()
	}
	return sanitizeTmuxCapture(out), ""
}

// tmuxCaptureFormat returns llm (default), human, or raw.
func tmuxCaptureFormat(args map[string]any) string {
	f, _ := args["format"].(string)
	f = strings.ToLower(strings.TrimSpace(f))
	switch f {
	case "human", "raw", "llm":
		return f
	case "full", "pretty":
		return "human"
	default:
		return "llm"
	}
}

// formatTmuxCapture applies Phase 2 LLM-optimized or human-readable shaping.
func formatTmuxCapture(format, session string, host *TmuxHost, lines int, body string) string {
	hostLabel := "localhost"
	if host != nil {
		hostLabel = host.Name
	}
	switch format {
	case "raw":
		return body
	case "human":
		cleaned := collapseBlankLines(trimRightLines(body), 2)
		header := fmt.Sprintf("=== tmux capture ===\nsession: %s\nhost: %s\nhistory_lines: %d\n---\n",
			session, hostLabel, lines)
		return header + cleaned
	default: // llm
		cleaned := collapseBlankLines(trimRightLines(body), 1)
		cleaned = strings.TrimSpace(cleaned)
		const maxLLM = 12000
		truncated := ""
		if len(cleaned) > maxLLM {
			cleaned = cleaned[len(cleaned)-maxLLM:]
			// snap to next newline if possible
			if i := strings.IndexByte(cleaned, '\n'); i > 0 && i < 200 {
				cleaned = cleaned[i+1:]
			}
			truncated = "\n[truncated to last 12000 chars]"
		}
		header := fmt.Sprintf("[tmux session=%s host=%s lines=%d]\n", session, hostLabel, lines)
		if cleaned == "" || cleaned == "(no output in pane)" {
			return header + "(no output in pane)"
		}
		return header + cleaned + truncated
	}
}

func trimRightLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t\r")
	}
	return strings.Join(lines, "\n")
}

// collapseBlankLines reduces runs of blank lines to at most maxBlank consecutive blanks.
func collapseBlankLines(s string, maxBlank int) string {
	if maxBlank < 0 {
		maxBlank = 0
	}
	lines := strings.Split(s, "\n")
	var out []string
	blankRun := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			blankRun++
			if blankRun <= maxBlank {
				out = append(out, "")
			}
			continue
		}
		blankRun = 0
		out = append(out, ln)
	}
	// trim trailing blanks
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

func requireTmuxLocalOrSSH(args map[string]any) error {
	name, _ := args["host"].(string)
	name = strings.TrimSpace(name)
	if name == "" || name == "local" || name == "localhost" {
		return requireTmux()
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("tmux_tools: ssh not found on PATH (required for remote host=%s)", name)
	}
	return nil
}

func requireTmux() error {
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux_tools: tmux not found on PATH (install tmux to use persistent sessions)")
	}
	return nil
}

func tmuxSessionFromArgs(args map[string]any) (string, error) {
	session := defaultTmuxSession
	if s, ok := args["session_name"].(string); ok && strings.TrimSpace(s) != "" {
		session = strings.TrimSpace(s)
	}
	if !tmuxSessionNameRE.MatchString(session) {
		return "", fmt.Errorf("invalid session_name %q (use only letters, digits, _ and -)", session)
	}
	return session, nil
}

func tmuxLinesFromArgs(args map[string]any) int {
	lines := 100
	switch v := args["lines"].(type) {
	case float64:
		lines = int(v)
	case int:
		lines = v
	case int64:
		lines = int(v)
	}
	if lines < 1 {
		lines = 1
	}
	if lines > 10000 {
		lines = 10000
	}
	return lines
}

func sanitizeTmuxCapture(s string) string {
	if s == "" {
		return s
	}
	// Strip NULs and other C0 controls; keep tab/newline/CR and normal UTF-8 text.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == 0:
			continue
		case r == '\n' || r == '\r' || r == '	':
			b.WriteRune(r)
		case r < 32 || r == 127:
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func hostSuffix(host *TmuxHost) string {
	if host == nil {
		return " on localhost"
	}
	return " on host " + host.Name
}

func isTmuxUnreachable(out string, err error) bool {
	if err == nil {
		return false
	}
	msg := out + " " + err.Error()
	return strings.Contains(msg, "server exited unexpectedly") ||
		strings.Contains(msg, "No such file") ||
		strings.Contains(msg, "can't find") ||
		strings.Contains(msg, "no server")
}
