// Package prefcli resolves preferred modern CLI tools with fallback to POSIX utilities.
// Supported: fd, bat, eza, fzf, ast-grep, zoxide, delta (see DESIGN.md Preferred CLI Tools).
package prefcli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Tool identifies a preferred CLI replacement.
type Tool string

const (
	ToolFd      Tool = "fd"
	ToolBat     Tool = "bat"
	ToolEza     Tool = "eza"
	ToolFzf     Tool = "fzf"
	ToolAstGrep Tool = "ast-grep"
	ToolZoxide  Tool = "zoxide"
	ToolDelta   Tool = "delta"
)

// Distro is a normalized package-manager family.
type Distro string

const (
	DistroAlpine  Distro = "alpine"
	DistroDebian  Distro = "debian"
	DistroRedHat  Distro = "redhat"
	DistroUnknown Distro = "unknown"
)

var lookPath = exec.LookPath

// DetectDistro reads /etc/os-release and returns alpine, debian, or redhat.
func DetectDistro() Distro {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return DistroUnknown
	}
	content := strings.ToLower(string(data))
	switch {
	case strings.Contains(content, "id=alpine"):
		return DistroAlpine
	case strings.Contains(content, "id=debian"), strings.Contains(content, "id=ubuntu"),
		strings.Contains(content, "id=linuxmint"), strings.Contains(content, "id=pop"):
		return DistroDebian
	case strings.Contains(content, "id=fedora"), strings.Contains(content, "id=rhel"),
		strings.Contains(content, "id=centos"), strings.Contains(content, "id=rocky"),
		strings.Contains(content, "id=almalinux"), strings.Contains(content, "id=ol"):
		return DistroRedHat
	default:
		return DistroUnknown
	}
}

// Available reports whether a preferred binary is on PATH.
func Available(tool Tool) bool {
	_, ok := Resolve(tool)
	return ok
}

// Resolve returns the first matching binary for a preferred tool.
func Resolve(tool Tool) (string, bool) {
	sp, ok := specs[tool]
	if !ok {
		return "", false
	}
	for _, b := range sp.bins {
		if _, err := lookPath(b); err == nil {
			return b, true
		}
	}
	return "", false
}

// Status returns availability of all preferred tools.
func Status() map[Tool]bool {
	out := make(map[Tool]bool, len(specs))
	for t := range specs {
		out[t] = Available(t)
	}
	return out
}

// Run executes a preferred tool with args, falling back to POSIX when absent.
func Run(tool Tool, prefArgs []string, fallbackArgs map[string]string) (used string, output string, err error) {
	if bin, ok := Resolve(tool); ok {
		cmd := exec.Command(bin, prefArgs...)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err = cmd.Run()
		return bin, buf.String(), err
	}
	sp := specs[tool]
	fb := sp.fallback(fallbackArgs)
	if len(fb) == 0 {
		return "fallback", "", fmt.Errorf("%s not available and no POSIX fallback", tool)
	}
	cmd := exec.Command(fb[0], fb[1:]...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	return fb[0], buf.String(), err
}

// RunWithInput like Run but supplies stdin (for fzf/grep filter pipelines).
func RunWithInput(tool Tool, prefArgs []string, fallbackArgs map[string]string, stdin string) (used string, output string, err error) {
	if bin, ok := Resolve(tool); ok {
		cmd := exec.Command(bin, prefArgs...)
		cmd.Stdin = strings.NewReader(stdin)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		err = cmd.Run()
		return bin, buf.String(), err
	}
	sp := specs[tool]
	fb := sp.fallback(fallbackArgs)
	if len(fb) == 0 {
		return "fallback", "", fmt.Errorf("%s not available and no POSIX fallback", tool)
	}
	cmd := exec.Command(fb[0], fb[1:]...)
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err = cmd.Run()
	return fb[0], buf.String(), err
}

// Pipe runs git diff (or other) through delta when available.
func PipeGitDiff(extraArgs []string) (used string, output string, err error) {
	gitArgs := append([]string{"diff"}, extraArgs...)
	git := exec.Command("git", gitArgs...)
	gitOut, err := git.CombinedOutput()
	if len(gitOut) == 0 && err != nil {
		return "git", "", err
	}
	if bin, ok := Resolve(ToolDelta); ok {
		cmd := exec.Command(bin)
		cmd.Stdin = bytes.NewReader(gitOut)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			return bin, string(gitOut), nil
		}
		return bin, buf.String(), nil
	}
	return "git", string(gitOut), nil
}

// MissingInstallPackages returns package names to install missing tools for this distro.
func MissingInstallPackages(d Distro) []string {
	var pkgs []string
	seen := make(map[string]bool)
	for tool, sp := range specs {
		if Available(tool) {
			continue
		}
		pkg, ok := sp.packages[d]
		if !ok || pkg == "" {
			continue
		}
		if !seen[pkg] {
			seen[pkg] = true
			pkgs = append(pkgs, pkg)
		}
	}
	return pkgs
}

// InstallCommand builds the distro-appropriate install shell command for packages.
func InstallCommand(d Distro, packages []string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("no packages to install")
	}
	if d == DistroUnknown {
		return "", fmt.Errorf("unsupported or unknown distro for automatic install")
	}
	pkgs := strings.Join(packages, " ")
	switch d {
	case DistroAlpine:
		return fmt.Sprintf("apk add --no-cache %s", pkgs), nil
	case DistroDebian:
		return fmt.Sprintf("apt-get update && apt-get install -y %s", pkgs), nil
	case DistroRedHat:
		if _, err := lookPath("dnf"); err == nil {
			return fmt.Sprintf("dnf install -y %s", pkgs), nil
		}
		return fmt.Sprintf("yum install -y %s", pkgs), nil
	default:
		return "", fmt.Errorf("unsupported distro: %s", d)
	}
}

// AllTools returns every managed preferred tool id.
func AllTools() []Tool {
	tools := make([]Tool, 0, len(specs))
	for t := range specs {
		tools = append(tools, t)
	}
	return tools
}
