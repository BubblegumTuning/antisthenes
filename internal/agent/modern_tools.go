package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nanami/antisthenes/internal/agent/prefcli"
)

func registerModernTools(r *ToolRegistry) {
	r.Register("modern_cli_status", func(args map[string]any) (string, error) {
		out := executeToolStatus(map[string]any{"tool": "prefcli"})
		return "Note: modern_cli_status is deprecated; use tool_status with tool=prefcli or tool=all.\n" + out, nil
	})

	r.Register("install_modern_cli", func(args map[string]any) (string, error) {
		res, err := executeInstallTool(r, "install_modern_cli", map[string]any{"tool": "prefcli_missing"})
		if err != nil {
			return "", err
		}
		note := "Note: install_modern_cli is deprecated; use install_tool with tool=prefcli_missing or tool=all_missing.\n"
		if res == "install_modern_cli: nothing to install" {
			return note + "All preferred CLI tools are already available on PATH.", nil
		}
		return note + res, nil
	})

	r.Register("find_files", func(args map[string]any) (string, error) {
		pattern, _ := args["pattern"].(string)
		path, _ := args["path"].(string)
		if path == "" {
			path = "."
		}
		prefArgs := []string{pattern, path}
		if pattern == "" {
			prefArgs = []string{path}
		}
		used, out, err := prefcli.Run(prefcli.ToolFd, prefArgs, map[string]string{
			"pattern": pattern,
			"path":    path,
		})
		if err != nil && out == "" {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Sprintf("find_files: no matches (via %s)", used), nil
		}
		return fmt.Sprintf("find_files (via %s):\n%s", used, out), nil
	})

	r.Register("cd_path", func(args map[string]any) (string, error) {
		query, _ := args["query"].(string)
		path, _ := args["path"].(string)
		target := strings.TrimSpace(query)
		if target == "" {
			target = strings.TrimSpace(path)
		}
		if target == "" {
			return "cd_path: query or path is required", nil
		}
		if err := validateRelativePath(target); err == nil {
			if info, statErr := os.Stat(target); statErr == nil && info.IsDir() {
				if chdirErr := os.Chdir(target); chdirErr != nil {
					return "", chdirErr
				}
				if bin, ok := prefcli.Resolve(prefcli.ToolZoxide); ok {
					_ = exec.Command(bin, "add", target).Run()
				}
				cwd, _ := os.Getwd()
				return fmt.Sprintf("cd_path (direct): now in %s", cwd), nil
			}
		}
		if bin, ok := prefcli.Resolve(prefcli.ToolZoxide); ok {
			cmd := exec.Command(bin, "query", target)
			out, err := cmd.CombinedOutput()
			if err == nil {
				resolved := strings.TrimSpace(string(out))
				if resolved != "" {
					if chdirErr := os.Chdir(resolved); chdirErr != nil {
						return "", chdirErr
					}
					_ = exec.Command(bin, "add", resolved).Run()
					cwd, _ := os.Getwd()
					return fmt.Sprintf("cd_path (via %s): now in %s", bin, cwd), nil
				}
			}
		}
		return fmt.Sprintf("cd_path: no directory match for %q (install zoxide or use a relative path)", target), nil
	})

	r.Register("resolve_path", func(args map[string]any) (string, error) {
		query, ok := args["query"].(string)
		if !ok || strings.TrimSpace(query) == "" {
			return "resolve_path: query is required", nil
		}
		query = strings.TrimSpace(query)
		if bin, ok := prefcli.Resolve(prefcli.ToolZoxide); ok {
			cmd := exec.Command(bin, "query", query)
			out, err := cmd.CombinedOutput()
			if err == nil && len(strings.TrimSpace(string(out))) > 0 {
				return fmt.Sprintf("resolve_path (via %s): %s", bin, strings.TrimSpace(string(out))), nil
			}
		}
		if info, err := os.Stat(query); err == nil {
			if info.IsDir() {
				return "resolve_path (fallback): " + query, nil
			}
		}
		return fmt.Sprintf("resolve_path: no match for %q (install zoxide or use absolute path)", query), nil
	})

	r.Register("fuzzy_find", func(args map[string]any) (string, error) {
		query, _ := args["query"].(string)
		path, _ := args["path"].(string)
		if path == "" {
			path = "."
		}
		// Build candidate list with fd/find, filter with fzf or grep.
		usedFind, candidates, err := prefcli.Run(prefcli.ToolFd, []string{".", path}, map[string]string{"path": path})
		if err != nil && candidates == "" {
			entries, rerr := os.ReadDir(path)
			if rerr != nil {
				return "", rerr
			}
			var names []string
			for _, e := range entries {
				names = append(names, e.Name())
			}
			candidates = strings.Join(names, "\n")
			usedFind = "readdir"
		}
		if query == "" {
			return fmt.Sprintf("fuzzy_find (via %s):\n%s", usedFind, candidates), nil
		}
		usedFilter, out, err := prefcli.RunWithInput(prefcli.ToolFzf,
			[]string{"--filter", query},
			map[string]string{"query": query},
			candidates,
		)
		if err != nil && out == "" {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Sprintf("fuzzy_find: no matches for %q (find=%s filter=%s)", query, usedFind, usedFilter), nil
		}
		return fmt.Sprintf("fuzzy_find (find=%s filter=%s):\n%s", usedFind, usedFilter, out), nil
	})
}

// readFilePreferred uses bat when available, else os.ReadFile.
func readFilePreferred(path string) (string, string, error) {
	used, out, err := prefcli.Run(prefcli.ToolBat,
		[]string{"--plain", "--paging=never", "--color=never", path},
		map[string]string{"path": path},
	)
	if err == nil && out != "" {
		return used, out, nil
	}
	data, rerr := os.ReadFile(path)
	if rerr != nil {
		if out != "" {
			return used, out, nil
		}
		return "read", "", rerr
	}
	return "read", string(data), nil
}

// listDirPreferred uses eza when available, else ls.
func listDirPreferred(path string) (string, string, error) {
	if path == "" {
		path = "."
	}
	used, out, err := prefcli.Run(prefcli.ToolEza,
		[]string{"--oneline", "--icons=never", path},
		map[string]string{"path": path},
	)
	if err != nil && out == "" {
		return used, "", err
	}
	return used, out, nil
}

// searchContentPreferred tries rg, then ast-grep, then grep -r.
func searchContentPreferred(pattern, path string) (string, string, error) {
	if path == "" {
		path = "."
	}
	if used, out, err := searchWithRipgrep(pattern, path); err == nil || out != "" {
		return used, out, err
	}
	used, out, err := prefcli.Run(prefcli.ToolAstGrep,
		[]string{"run", "-p", pattern, path},
		map[string]string{"pattern": pattern, "path": path},
	)
	if err != nil && out == "" {
		return used, "", err
	}
	return used, out, nil
}

func searchWithRipgrep(pattern, path string) (string, string, error) {
	bin, err := exec.LookPath("rg")
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command(bin, "--line-number", "--no-heading", "--color=never", pattern, path)
	out, err := cmd.CombinedOutput()
	return bin, string(out), err
}
