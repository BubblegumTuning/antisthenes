package agent

import (
	"fmt"
	"os"
	"strings"
)

func registerSearchPatchTools(r *ToolRegistry) {
	r.Register("search_files", func(args map[string]any) (string, error) {
		pattern, ok := args["pattern"].(string)
		if !ok || pattern == "" {
			return "search_files: pattern is required", nil
		}
		searchPath, _ := args["path"].(string)
		if searchPath == "" {
			searchPath = "."
		}
		maxResults := 500
		switch v := args["max_results"].(type) {
		case float64:
			maxResults = int(v)
		case int:
			maxResults = v
		}
		const hardCap = 5000
		if maxResults <= 0 {
			maxResults = hardCap
		}
		if maxResults > hardCap {
			maxResults = hardCap
		}

		used, out, err := searchContentPreferred(pattern, searchPath)
		if err != nil && out == "" {
			return "", err
		}
		trimmed := strings.TrimSpace(out)
		if trimmed == "" {
			return fmt.Sprintf("search_files: no matches for pattern '%s' under %s", pattern, searchPath), nil
		}
		lines := strings.Split(trimmed, "\n")
		if len(lines) > maxResults {
			lines = lines[:maxResults]
			out = strings.Join(lines, "\n") + fmt.Sprintf("\n... (truncated to %d lines)", maxResults)
		} else {
			out = trimmed
		}
		return fmt.Sprintf("search_files (via %s):\n%s", used, out), nil
	})

	r.Register("patch", func(args map[string]any) (string, error) {
		diff, _ := args["diff"].(string)
		path, _ := args["path"].(string)
		oldText, _ := args["old_text"].(string)
		newText, _ := args["new_text"].(string)

		if strings.TrimSpace(diff) != "" {
			method, err := applyPatchDiff(diff, strings.TrimSpace(path))
			if err != nil {
				return "patch: " + err.Error(), nil
			}
			target := strings.TrimSpace(path)
			if target == "" {
				if t, _, perr := parseUnifiedDiff(diff); perr == nil {
					target = t
				}
			}
			if target == "" {
				return fmt.Sprintf("patch: applied via %s", method), nil
			}
			return fmt.Sprintf("patch: applied unified diff via %s to %s", method, target), nil
		}

		if path == "" {
			return "patch: provide diff (unified) or path + old_text + new_text", nil
		}
		if oldText != "" && newText != "" {
			data, err := os.ReadFile(path)
			if err != nil {
				return "patch read error: " + err.Error(), nil
			}
			updated := strings.Replace(string(data), oldText, newText, -1)
			err = os.WriteFile(path, []byte(updated), 0644)
			if err != nil {
				return "patch write error: " + err.Error(), nil
			}
			return "patch: applied string replace to " + path, nil
		}
		return "patch: provide diff (unified) or path + old_text + new_text", nil
	})
}
