package installable

import (
	"strings"
	"testing"

	"github.com/nanami/antisthenes/internal/agent/prefcli"
)

func TestResolveID(t *testing.T) {
	cases := map[string]string{
		"rg": "rg", "ripgrep": "rg", "fd": "fd", "fdfind": "fd", "nmap": "nmap",
		"ast-grep": "ast-grep", "sg": "ast-grep", "all_missing": "all_missing",
		"prefcli_missing": "prefcli_missing", "prefcli": "prefcli",
	}
	for in, want := range cases {
		got, ok := ResolveID(in)
		if !ok || got != want {
			t.Fatalf("ResolveID(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	if _, ok := ResolveID("not-a-tool"); ok {
		t.Fatal("expected unknown tool")
	}
}

func TestPackagesFor(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(string) (string, error) { return "", errNotFound }

	pkgs, err := PackagesFor([]string{"fd", "rg"}, prefcli.DistroDebian)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 2 || !containsAll(pkgs, "fd-find", "ripgrep") {
		t.Fatalf("unexpected packages: %v", pkgs)
	}
}

func TestInstallShellCommand(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(string) (string, error) { return "", errNotFound }

	cmd, err := InstallShellCommand([]string{"rg", "fd"}, nil, prefcli.DistroRedHat)
	if err != nil || !strings.Contains(cmd, "dnf install -y") || !strings.Contains(cmd, "ripgrep") || !strings.Contains(cmd, "fd") {
		t.Fatalf("redhat: %v %q", err, cmd)
	}
	cmd, err = InstallShellCommand(nil, []string{"ansible"}, prefcli.DistroDebian)
	if err != nil || !strings.Contains(cmd, "python3 -m venv .ansible-venv") {
		t.Fatalf("venv: %v %q", err, cmd)
	}
}

func TestPrefcliMissingSubset(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(string) (string, error) { return "", errNotFound }

	pkgIDs, venvIDs, _, _, _, err := BuildInstallPlan([]string{"prefcli_missing"}, prefcli.DistroDebian)
	if err != nil {
		t.Fatal(err)
	}
	if containsString(pkgIDs, "rg") || containsString(pkgIDs, "nmap") {
		t.Fatalf("prefcli_missing should not include rg/nmap: %v", pkgIDs)
	}
	if !containsString(pkgIDs, "fd") {
		t.Fatalf("expected fd in prefcli_missing plan: %v", pkgIDs)
	}
	if len(venvIDs) != 0 {
		t.Fatalf("prefcli_missing should not include venv tools: %v", venvIDs)
	}
}

func TestBuildInstallPlanAllMissing(t *testing.T) {
	orig := lookPath
	defer func() { lookPath = orig }()
	lookPath = func(string) (string, error) { return "", errNotFound }

	pkgIDs, venvIDs, _, _, _, err := BuildInstallPlan([]string{"all_missing"}, prefcli.DistroAlpine)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgIDs) == 0 {
		t.Fatal("expected missing pkgmgr tools")
	}
	if !containsString(pkgIDs, "rg") || !containsString(pkgIDs, "fd") {
		t.Fatalf("expected rg and fd in plan: %v", pkgIDs)
	}
	if !containsString(venvIDs, "ansible") {
		t.Fatalf("expected ansible in venv plan: %v", venvIDs)
	}
}

func TestFormatStatusUnknown(t *testing.T) {
	out := FormatStatus(prefcli.DistroDebian, "nope")
	if !strings.Contains(out, "unknown tool") {
		t.Fatalf("unexpected: %s", out)
	}
}

func containsAll(slice []string, items ...string) bool {
	for _, item := range items {
		if !containsString(slice, item) {
			return false
		}
	}
	return true
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

var errNotFound = &pathError{}

type pathError struct{}

func (e *pathError) Error() string { return "not found" }
