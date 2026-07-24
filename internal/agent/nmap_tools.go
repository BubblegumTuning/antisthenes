package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nanami/antisthenes/internal/agent/installable"
)

var (
	hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?$`)
	portsPattern    = regexp.MustCompile(`^\d{1,5}(-\d{1,5})?(,\d{1,5}(-\d{1,5})?)*$`)
)

const (
	nmapDefaultTimeout = 120
	nmapMaxTimeout     = 600
	nmapDefaultPorts   = "1-1000"
	nmapLogDir         = "logs/nmap"
)

var resolveNmapBinary = defaultResolveNmapBinary

// RegisterNmapTools adds nmap_scan when enabled is true.
func RegisterNmapTools(r *ToolRegistry, enabled bool) {
	if !enabled {
		return
	}
	r.Register("nmap_scan", func(args map[string]any) (string, error) {
		target, ok := args["target"].(string)
		if !ok || strings.TrimSpace(target) == "" {
			return "nmap_scan: target is required", nil
		}
		target = strings.TrimSpace(target)
		if err := validateNmapTarget(target); err != nil {
			return "nmap_scan: " + err.Error(), nil
		}

		bin, err := resolveNmapBinary()
		if err != nil {
			return "nmap_scan: nmap not found on PATH. Run tool_status, then install_tool with tool=nmap.", nil
		}

		scanType, _ := args["scan_type"].(string)
		scanType = strings.TrimSpace(strings.ToLower(scanType))
		if scanType == "" {
			scanType = "ping"
		}

		ports, _ := args["ports"].(string)
		ports = strings.TrimSpace(ports)
		if ports == "" && (scanType == "ports" || scanType == "service") {
			ports = nmapDefaultPorts
		}
		if ports != "" {
			if err := validateNmapPorts(ports); err != nil {
				return "nmap_scan: " + err.Error(), nil
			}
		}

		nmapArgs, err := buildNmapArgs(scanType, target, ports)
		if err != nil {
			return "nmap_scan: " + err.Error(), nil
		}

		timeoutSec := parseNmapTimeout(args["timeout"])
		cmdStr := bin + " " + strings.Join(nmapArgs, " ")
		if ok, denied := r.requestInteractiveApproval("nmap_scan", cmdStr); !ok {
			if denied {
				return "nmap_scan: denied by user", nil
			}
			return "nmap_scan: approval required. Use approve_tool or approve via TUI popup.", nil
		}

		_ = os.MkdirAll(nmapLogDir, 0o755)
		ts := time.Now().Unix()
		safeName := strings.NewReplacer("/", "_", ":", "_", ".", "_").Replace(target)
		logPath := filepath.Join(nmapLogDir, fmt.Sprintf("scan-%s-%d.log", safeName, ts))

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, bin, nmapArgs...)
		out, runErr := cmd.CombinedOutput()
		logContent := string(out)
		if runErr != nil {
			if ctx.Err() == context.DeadlineExceeded {
				logContent += fmt.Sprintf("\nError: timed out after %ds", timeoutSec)
			} else {
				logContent += "\nError: " + runErr.Error()
			}
		}
		_ = os.WriteFile(logPath, []byte(logContent), 0o644)

		tail := tailLines(logContent, 20)
		summary := fmt.Sprintf("nmap_scan: %s scan of %s\nLog: %s\n--- tail ---\n%s", scanType, target, logPath, tail)
		if runErr != nil && ctx.Err() != context.DeadlineExceeded {
			return summary, nil
		}
		return summary, nil
	})
}

func defaultResolveNmapBinary() (string, error) {
	e, ok := installable.Get("nmap")
	if !ok {
		return "", fmt.Errorf("nmap not in catalog")
	}
	if bin, ok := installable.ResolveBin(e); ok {
		return bin, nil
	}
	return "", fmt.Errorf("nmap not found")
}

func validateNmapTarget(target string) error {
	if strings.ContainsAny(target, ";&|$`<>\"'\\") {
		return fmt.Errorf("invalid target: disallowed characters")
	}
	lower := strings.ToLower(target)
	if lower == "0.0.0.0/0" || lower == "::/0" {
		return fmt.Errorf("refusing to scan entire internet range")
	}
	if ip := net.ParseIP(target); ip != nil {
		return nil
	}
	if _, _, err := net.ParseCIDR(target); err == nil {
		return nil
	}
	if hostnamePattern.MatchString(target) {
		return nil
	}
	return fmt.Errorf("invalid target %q (use hostname, IP, or CIDR)", target)
}

func validateNmapPorts(ports string) error {
	if !portsPattern.MatchString(ports) {
		return fmt.Errorf("invalid ports %q (use e.g. 22,80,443 or 1-1000)", ports)
	}
	for _, part := range strings.Split(ports, ",") {
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				return fmt.Errorf("invalid port range %q", part)
			}
			lo, hi := parsePort(bounds[0]), parsePort(bounds[1])
			if lo == 0 || hi == 0 || lo > hi || hi > 65535 {
				return fmt.Errorf("invalid port range %q", part)
			}
			continue
		}
		if p := parsePort(part); p == 0 || p > 65535 {
			return fmt.Errorf("invalid port %q", part)
		}
	}
	return nil
}

func parsePort(s string) int {
	var p int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		p = p*10 + int(c-'0')
	}
	return p
}

func buildNmapArgs(scanType, target, ports string) ([]string, error) {
	switch scanType {
	case "ping":
		return []string{"-sn", target}, nil
	case "quick":
		return []string{"-F", target}, nil
	case "ports":
		if ports == "" {
			return nil, fmt.Errorf("ports required for scan_type=ports")
		}
		return []string{"-sT", "-p", ports, target}, nil
	case "service":
		if ports == "" {
			return nil, fmt.Errorf("ports required for scan_type=service")
		}
		return []string{"-sT", "-sV", "-p", ports, target}, nil
	default:
		return nil, fmt.Errorf("unknown scan_type %q (use ping, quick, ports, or service)", scanType)
	}
}

func parseNmapTimeout(v any) int {
	switch t := v.(type) {
	case float64:
		if int(t) > 0 {
			return capNmapTimeout(int(t))
		}
	case int:
		if t > 0 {
			return capNmapTimeout(t)
		}
	}
	return nmapDefaultTimeout
}

func capNmapTimeout(sec int) int {
	if sec > nmapMaxTimeout {
		return nmapMaxTimeout
	}
	return sec
}

func tailLines(content string, n int) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) <= n {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
