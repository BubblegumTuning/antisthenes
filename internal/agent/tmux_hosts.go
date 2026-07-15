package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// TmuxHost is a registered remote host for Phase 1+ SSH-backed tmux tools.
type TmuxHost struct {
	Name        string `json:"name"`                   // registry alias (used as host= tool arg)
	Host        string `json:"host"`                   // hostname or IP
	User        string `json:"user"`                   // SSH user
	KeyPath     string `json:"key_path"`               // private key path (absolute preferred)
	SessionName string `json:"session_name,omitempty"` // default remote tmux session name
	Port        int    `json:"port,omitempty"`         // SSH port (default 22)
}

type tmuxHostRegistry struct {
	Hosts []TmuxHost `json:"hosts"`
}

var (
	tmuxHostsMu   sync.Mutex
	tmuxHostsFile = "tmux_hosts.json" // relative to process cwd unless SetTmuxHostsPath used
)

// SetTmuxHostsPath overrides the host registry file path (tests / custom layouts).
func SetTmuxHostsPath(path string) {
	tmuxHostsMu.Lock()
	defer tmuxHostsMu.Unlock()
	if strings.TrimSpace(path) == "" {
		tmuxHostsFile = "tmux_hosts.json"
		return
	}
	tmuxHostsFile = path
}

func getTmuxHostsPath() string {
	return tmuxHostsFile
}

func loadTmuxHosts() (tmuxHostRegistry, error) {
	path := getTmuxHostsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return tmuxHostRegistry{Hosts: nil}, nil
		}
		return tmuxHostRegistry{}, err
	}
	var reg tmuxHostRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return tmuxHostRegistry{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return reg, nil
}

func saveTmuxHosts(reg tmuxHostRegistry) error {
	path := getTmuxHostsPath()
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func findTmuxHost(name string) (*TmuxHost, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "localhost" || name == "local" {
		return nil, nil // local execution
	}
	reg, err := loadTmuxHosts()
	if err != nil {
		return nil, err
	}
	for i := range reg.Hosts {
		if reg.Hosts[i].Name == name {
			h := reg.Hosts[i]
			return &h, nil
		}
	}
	return nil, fmt.Errorf("unknown host %q (use tmux_register_host / tmux_list_hosts)", name)
}

func upsertTmuxHost(h TmuxHost) error {
	tmuxHostsMu.Lock()
	defer tmuxHostsMu.Unlock()
	reg, err := loadTmuxHosts()
	if err != nil {
		return err
	}
	found := false
	for i := range reg.Hosts {
		if reg.Hosts[i].Name == h.Name {
			reg.Hosts[i] = h
			found = true
			break
		}
	}
	if !found {
		reg.Hosts = append(reg.Hosts, h)
	}
	return saveTmuxHosts(reg)
}

func expandHomePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// validateTmuxHostKey checks the private key file exists and is a regular file.
func validateTmuxHostKey(keyPath string) error {
	st, err := os.Stat(keyPath)
	if err != nil {
		return fmt.Errorf("key_path %q: %w", keyPath, err)
	}
	if st.IsDir() {
		return fmt.Errorf("key_path %q is a directory", keyPath)
	}
	return nil
}

// validateTmuxHostSSH runs a quick BatchMode SSH connectivity check.
func validateTmuxHostSSH(h TmuxHost) error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("ssh not found on PATH")
	}
	port := h.Port
	if port <= 0 {
		port = 22
	}
	args := []string{
		"-i", h.KeyPath,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=8",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(port),
		fmt.Sprintf("%s@%s", h.User, h.Host),
		"true",
	}
	cmd := exec.Command("ssh", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ssh check failed: %s", msg)
	}
	return nil
}

// runTmux runs a tmux subcommand locally or over SSH for a registered host.
// args are tmux argv after "tmux" (e.g. "has-session", "-t", "name").
func runTmux(host *TmuxHost, args ...string) (stdout string, err error) {
	if host == nil {
		if _, e := exec.LookPath("tmux"); e != nil {
			return "", fmt.Errorf("tmux not found on PATH")
		}
		cmd := exec.Command("tmux", args...)
		out, e := cmd.CombinedOutput()
		return string(out), e
	}
	if _, e := exec.LookPath("ssh"); e != nil {
		return "", fmt.Errorf("ssh not found on PATH")
	}
	port := host.Port
	if port <= 0 {
		port = 22
	}
	sshArgs := []string{
		"-i", host.KeyPath,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=15",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", strconv.Itoa(port),
		fmt.Sprintf("%s@%s", host.User, host.Host),
		"tmux",
	}
	sshArgs = append(sshArgs, args...)
	cmd := exec.Command("ssh", sshArgs...)
	out, e := cmd.CombinedOutput()
	return string(out), e
}

func hostFromArgs(args map[string]any) (*TmuxHost, error) {
	name, _ := args["host"].(string)
	return findTmuxHost(name)
}

// sessionNameForHost picks session: explicit session_name arg, else host default, else global default.
func sessionNameForHost(args map[string]any, host *TmuxHost) (string, error) {
	if s, ok := args["session_name"].(string); ok && strings.TrimSpace(s) != "" {
		return tmuxSessionFromArgs(args)
	}
	if host != nil && strings.TrimSpace(host.SessionName) != "" {
		session := strings.TrimSpace(host.SessionName)
		if !tmuxSessionNameRE.MatchString(session) {
			return "", fmt.Errorf("invalid host session_name %q", session)
		}
		return session, nil
	}
	return tmuxSessionFromArgs(args)
}

func hostAliasREMatch(name string) bool {
	return tmuxSessionNameRE.MatchString(name) // same safe charset
}

func formatHostList(reg tmuxHostRegistry) string {
	if len(reg.Hosts) == 0 {
		return "no registered tmux hosts"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("registered tmux hosts (%d):\n", len(reg.Hosts)))
	for _, h := range reg.Hosts {
		port := h.Port
		if port <= 0 {
			port = 22
		}
		sess := h.SessionName
		if sess == "" {
			sess = defaultTmuxSession
		}
		b.WriteString(fmt.Sprintf("- %s: %s@%s:%d key=%s session=%s\n",
			h.Name, h.User, h.Host, port, h.KeyPath, sess))
	}
	return strings.TrimSpace(b.String())
}
