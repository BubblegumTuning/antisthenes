package prefcli

import (
	"strings"
	"testing"
)

func TestInstallCommand(t *testing.T) {
	cmd, err := InstallCommand(DistroAlpine, []string{"fd", "bat"})
	if err != nil || !strings.Contains(cmd, "apk add") || !strings.Contains(cmd, "fd") {
		t.Fatalf("alpine: %v %q", err, cmd)
	}
	cmd, err = InstallCommand(DistroDebian, []string{"fd-find"})
	if err != nil || !strings.Contains(cmd, "apt-get install") {
		t.Fatalf("debian: %v %q", err, cmd)
	}
	cmd, err = InstallCommand(DistroRedHat, []string{"git-delta"})
	if err != nil || !strings.Contains(cmd, "install -y") {
		t.Fatalf("redhat: %v %q", err, cmd)
	}
	_, err = InstallCommand(DistroUnknown, []string{"fd"})
	if err == nil {
		t.Fatal("expected error for unknown distro")
	}
}

func TestResolveFallback(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(name string) (string, error) {
		if name == "eza" {
			return "/usr/bin/eza", nil
		}
		return "", errNotFound
	}
	bin, ok := Resolve(ToolEza)
	if !ok || bin != "eza" {
		t.Fatalf("resolve eza: %q %v", bin, ok)
	}
	_, ok = Resolve(ToolFd)
	if ok {
		t.Fatal("fd should be missing")
	}
}

func TestRunFallbackLs(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(string) (string, error) { return "", errNotFound }
	used, out, err := Run(ToolEza, []string{"--oneline", "/"}, map[string]string{"path": "/"})
	if err != nil {
		t.Fatalf("run fallback: %v", err)
	}
	if used != "ls" || out == "" {
		t.Fatalf("unexpected fallback: %s %q", used, out)
	}
}

var errNotFound = &pathError{}

type pathError struct{}

func (e *pathError) Error() string { return "not found" }
