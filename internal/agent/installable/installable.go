package installable

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nanami/antisthenes/internal/agent/prefcli"
)

var lookPath = exec.LookPath

func init() {
	byID = make(map[string]Entry, len(catalog))
	for _, e := range catalog {
		byID[e.ID] = e
	}
}

var byID map[string]Entry

// PrefcliIDs are catalog ids that back prefcli preferred CLI tools (fd, bat, eza, etc.).
var PrefcliIDs = []string{"fd", "bat", "eza", "fzf", "ast-grep", "zoxide", "delta"}

// AllIDs returns sorted canonical tool ids from the catalog.
func AllIDs() []string {
	ids := make([]string, 0, len(catalog))
	for _, e := range catalog {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)
	return ids
}

// ResolveID maps a tool name or alias to a canonical catalog id.
func ResolveID(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "all" {
		return "", false
	}
	if name == "all_missing" || name == "prefcli_missing" || name == "prefcli" {
		return name, true
	}
	for _, e := range catalog {
		if name == e.ID {
			return e.ID, true
		}
		for _, a := range e.Aliases {
			if name == strings.ToLower(a) {
				return e.ID, true
			}
		}
	}
	return "", false
}

// Get returns a catalog entry by canonical id.
func Get(id string) (Entry, bool) {
	e, ok := byID[id]
	return e, ok
}

// Available reports whether any binary for the entry is on PATH.
func Available(e Entry) bool {
	_, ok := ResolveBin(e)
	return ok
}

// ResolveBin returns the first matching binary for an entry.
func ResolveBin(e Entry) (string, bool) {
	for _, b := range e.Bins {
		if p, err := lookPath(b); err == nil {
			return p, true
		}
	}
	if e.Strategy == StrategyVenvPip && e.VenvDir != "" {
		for _, b := range e.Bins {
			venvBin := filepath.Join(e.VenvDir, "bin", b)
			if _, err := os.Stat(venvBin); err == nil {
				return venvBin, true
			}
		}
	}
	return "", false
}

// MissingPrefcliIDs returns missing prefcli-backed tools installable via the package manager.
func MissingPrefcliIDs(d prefcli.Distro) []string {
	return missingFromIDs(PrefcliIDs, d)
}

// MissingPkgMgrIDs returns catalog ids that use the package manager and are not on PATH.
func MissingPkgMgrIDs(d prefcli.Distro) []string {
	var missing []string
	for _, e := range catalog {
		if e.Strategy != StrategyPkgMgr || Available(e) {
			continue
		}
		if pkg, ok := e.Packages[d]; !ok || pkg == "" {
			continue
		}
		missing = append(missing, e.ID)
	}
	sort.Strings(missing)
	return missing
}

// PackagesFor returns deduplicated package names for the given ids on distro d.
func PackagesFor(ids []string, d prefcli.Distro) ([]string, error) {
	seen := make(map[string]bool)
	var pkgs []string
	for _, id := range ids {
		e, ok := Get(id)
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", id)
		}
		if e.Strategy != StrategyPkgMgr {
			continue
		}
		if Available(e) {
			continue
		}
		pkg, ok := e.Packages[d]
		if !ok || pkg == "" {
			return nil, fmt.Errorf("no package mapping for %s on %s", id, d)
		}
		if !seen[pkg] {
			seen[pkg] = true
			pkgs = append(pkgs, pkg)
		}
	}
	sort.Strings(pkgs)
	return pkgs, nil
}

// VenvPipCommand builds a shell command to create a venv and pip-install a package.
func VenvPipCommand(e Entry) (string, error) {
	if e.Strategy != StrategyVenvPip {
		return "", fmt.Errorf("%s is not a venv-pip tool", e.ID)
	}
	if e.VenvDir == "" || e.PipPackage == "" {
		return "", fmt.Errorf("%s: incomplete venv-pip config", e.ID)
	}
	pip := filepath.Join(e.VenvDir, "bin", "pip")
	return fmt.Sprintf("python3 -m venv %s && %s install %s", e.VenvDir, pip, e.PipPackage), nil
}

// BuildInstallPlan resolves requested tool names into executable install steps.
func BuildInstallPlan(requested []string, d prefcli.Distro) (pkgIDs []string, venvIDs []string, manual []string, already []string, unknown []string, err error) {
	ids, err := expandRequested(requested, d)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	if len(ids) == 0 {
		return nil, nil, nil, nil, nil, nil
	}
	for _, id := range ids {
		e, ok := Get(id)
		if !ok {
			unknown = append(unknown, id)
			continue
		}
		if Available(e) {
			already = append(already, id)
			continue
		}
		switch e.Strategy {
		case StrategyPkgMgr:
			if pkg, ok := e.Packages[d]; !ok || pkg == "" {
				manual = append(manual, id+": no package for "+string(d))
			} else {
				pkgIDs = append(pkgIDs, id)
			}
		case StrategyVenvPip:
			venvIDs = append(venvIDs, id)
		case StrategyManualOnly:
			manual = append(manual, id+": "+e.ManualHint)
		}
	}
	pkgIDs = dedupeSorted(pkgIDs)
	venvIDs = dedupeSorted(venvIDs)
	already = dedupeSorted(already)
	unknown = dedupeSorted(unknown)
	sort.Strings(manual)
	return pkgIDs, venvIDs, manual, already, unknown, nil
}

// InstallShellCommand builds a single shell command for pkgmgr + venv steps.
func InstallShellCommand(pkgIDs, venvIDs []string, d prefcli.Distro) (string, error) {
	var parts []string
	if len(pkgIDs) > 0 {
		pkgs, err := PackagesFor(pkgIDs, d)
		if err != nil {
			return "", err
		}
		if len(pkgs) == 0 {
			return "", fmt.Errorf("no packages to install")
		}
		cmd, err := prefcli.InstallCommand(d, pkgs)
		if err != nil {
			return "", err
		}
		parts = append(parts, cmd)
	}
	for _, id := range venvIDs {
		e, ok := Get(id)
		if !ok {
			return "", fmt.Errorf("unknown tool %q", id)
		}
		venvCmd, err := VenvPipCommand(e)
		if err != nil {
			return "", err
		}
		parts = append(parts, venvCmd)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("nothing to install")
	}
	return strings.Join(parts, " && "), nil
}

// FormatStatus renders availability for one tool or the full catalog.
func FormatStatus(d prefcli.Distro, filterID string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Distro: %s\n", d))
	entries := catalog
	switch filterID {
	case "", "all":
		// full catalog
	case "prefcli":
		entries = entriesForIDs(PrefcliIDs)
	default:
		e, ok := Get(filterID)
		if !ok {
			return fmt.Sprintf("tool_status: unknown tool %q (known: %s)", filterID, strings.Join(AllIDs(), ", "))
		}
		entries = []Entry{e}
	}
	for _, e := range entries {
		if bin, ok := ResolveBin(e); ok {
			b.WriteString(fmt.Sprintf("  %s: %s (available)\n", e.ID, bin))
			continue
		}
		switch e.Strategy {
		case StrategyPkgMgr:
			pkg := e.Packages[d]
			if pkg == "" {
				b.WriteString(fmt.Sprintf("  %s: missing (no package mapping for %s)\n", e.ID, d))
			} else {
				b.WriteString(fmt.Sprintf("  %s: missing → package %s (install_tool: %s)\n", e.ID, pkg, e.ID))
			}
		case StrategyVenvPip:
			b.WriteString(fmt.Sprintf("  %s: missing → venv %s + pip install %s (install_tool: %s)\n", e.ID, e.VenvDir, e.PipPackage, e.ID))
		case StrategyManualOnly:
			b.WriteString(fmt.Sprintf("  %s: missing → manual install (%s)\n", e.ID, e.ManualHint))
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func expandRequested(requested []string, d prefcli.Distro) ([]string, error) {
	if len(requested) == 0 {
		return nil, fmt.Errorf("no tools requested")
	}
	var ids []string
	for _, raw := range requested {
		id, ok := ResolveID(raw)
		if !ok {
			return nil, fmt.Errorf("unknown tool %q", raw)
		}
		switch id {
		case "all_missing":
			ids = append(ids, MissingPkgMgrIDs(d)...)
			for _, e := range catalog {
				if e.Strategy == StrategyVenvPip && !Available(e) {
					ids = append(ids, e.ID)
				}
			}
		case "prefcli_missing":
			ids = append(ids, MissingPrefcliIDs(d)...)
		default:
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		for _, raw := range requested {
			id, ok := ResolveID(raw)
			if ok && (id == "all_missing" || id == "prefcli_missing") {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("no installable tools matched request")
	}
	return dedupeSorted(ids), nil
}

func missingFromIDs(toolIDs []string, d prefcli.Distro) []string {
	var missing []string
	for _, id := range toolIDs {
		e, ok := Get(id)
		if !ok || e.Strategy != StrategyPkgMgr || Available(e) {
			continue
		}
		if pkg, ok := e.Packages[d]; ok && pkg != "" {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	return missing
}

func entriesForIDs(toolIDs []string) []Entry {
	var entries []Entry
	for _, id := range toolIDs {
		if e, ok := Get(id); ok {
			entries = append(entries, e)
		}
	}
	return entries
}

func dedupeSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
