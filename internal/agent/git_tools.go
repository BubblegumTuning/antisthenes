package agent

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/nanami/antisthenes/internal/agent/prefcli"
)

func registerGitTools(r *ToolRegistry) {
	r.Register("git_status", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		gitArgs := []string{"status", "--short"}
		if full, _ := args["full"].(bool); full {
			gitArgs = []string{"status"}
		}
		if path, _ := args["path"].(string); strings.TrimSpace(path) != "" {
			gitArgs = append(gitArgs, "--", strings.TrimSpace(path))
		}
		out, err := runGit(cwd, gitArgs...)
		return formatGitOutput("git_status", out, err)
	})

	r.Register("git_log", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		count := 20
		switch v := args["count"].(type) {
		case float64:
			count = int(v)
		case int:
			count = v
		}
		if count <= 0 {
			count = 20
		}
		if count > 200 {
			count = 200
		}
		gitArgs := []string{"log", "--oneline", "-n", strconv.Itoa(count)}
		if path, _ := args["path"].(string); strings.TrimSpace(path) != "" {
			gitArgs = append(gitArgs, "--", strings.TrimSpace(path))
		}
		out, err := runGit(cwd, gitArgs...)
		return formatGitOutput("git_log", out, err)
	})

	r.Register("git_add", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		all, _ := args["all"].(bool)
		paths, _ := args["paths"].(string)
		paths = strings.TrimSpace(paths)

		var gitArgs []string
		if all {
			gitArgs = []string{"add", "-A"}
		} else if paths != "" {
			gitArgs = append([]string{"add"}, strings.Fields(paths)...)
		} else {
			return "git_add: provide paths or all=true", nil
		}

		return r.gitWithApproval("git_add", gitArgs, cwd)
	})

	r.Register("git_commit", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		message, ok := args["message"].(string)
		if !ok || strings.TrimSpace(message) == "" {
			return "git_commit: message is required", nil
		}
		message = strings.TrimSpace(message)
		gitArgs := []string{"commit", "-m", message}
		if all, _ := args["all"].(bool); all {
			gitArgs = append(gitArgs, "-a")
		}
		if amend, _ := args["amend"].(bool); amend {
			gitArgs = append(gitArgs, "--amend")
		}

		return r.gitWithApproval("git_commit", gitArgs, cwd)
	})

	r.Register("git_checkout", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		ref, ok := args["ref"].(string)
		if !ok || strings.TrimSpace(ref) == "" {
			return "git_checkout: ref is required (branch name, commit, or path)", nil
		}
		ref = strings.TrimSpace(ref)
		gitArgs := []string{"checkout", ref}
		if create, _ := args["create_branch"].(bool); create {
			gitArgs = []string{"checkout", "-b", ref}
		}

		return r.gitWithApproval("git_checkout", gitArgs, cwd)
	})

	r.Register("git_branch", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		name, _ := args["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			out, err := runGit(cwd, "branch", "-vv")
			return formatGitOutput("git_branch", out, err)
		}
		gitArgs := []string{"branch", name}
		return r.gitWithApproval("git_branch", gitArgs, cwd)
	})

	r.Register("git_show", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		ref, _ := args["ref"].(string)
		ref = strings.TrimSpace(ref)
		if ref == "" {
			ref = "HEAD"
		}
		stat, _ := args["stat"].(bool)
		gitArgs := []string{"show", "--format=fuller"}
		if stat {
			gitArgs = append(gitArgs, "--stat")
		}
		gitArgs = append(gitArgs, ref)
		out, err := runGit(cwd, gitArgs...)
		return formatGitOutput("git_show", out, err)
	})

	r.Register("git_diff", func(args map[string]any) (string, error) {
		cwd := gitCwd(args)
		extra, _ := args["args"].(string)
		var extraArgs []string
		if extra != "" {
			extraArgs = strings.Fields(extra)
		}
		if cwd != "" {
			// prefcli.PipeGitDiff doesn't support cwd; run git directly in cwd.
			gitArgs := append([]string{"diff"}, extraArgs...)
			out, err := runGit(cwd, gitArgs...)
			if err != nil && strings.TrimSpace(out) == "" {
				return "", err
			}
			if strings.TrimSpace(out) == "" {
				return "git_diff: no diff", nil
			}
			return fmt.Sprintf("git_diff (git):\n%s", strings.TrimSpace(out)), nil
		}
		used, out, err := prefcli.PipeGitDiff(extraArgs)
		if err != nil && out == "" {
			return "", err
		}
		if strings.TrimSpace(out) == "" {
			return fmt.Sprintf("git_diff: no diff (via %s)", used), nil
		}
		return fmt.Sprintf("git_diff (via %s):\n%s", used, out), nil
	})
}

func gitCwd(args map[string]any) string {
	cwd, _ := args["cwd"].(string)
	return strings.TrimSpace(cwd)
}

func runGit(cwd string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (r *ToolRegistry) gitWithApproval(tool string, args []string, cwd string) (string, error) {
	cmdStr := "git " + strings.Join(args, " ")
	if ok, denied := r.resolveApproval(tool, cmdStr); !ok {
		if denied {
			return tool + ": denied by user", nil
		}
		return tool + ": approval required. Use approve_tool or TUI popup.", nil
	}
	out, err := runGit(cwd, args...)
	return formatGitOutput(tool, out, err)
}

func formatGitOutput(tool string, out string, err error) (string, error) {
	out = strings.TrimSpace(out)
	if err != nil {
		if out == "" {
			return "", err
		}
		return tool + ":\n" + out + "\nError: " + err.Error(), nil
	}
	if out == "" {
		return tool + ": ok", nil
	}
	return tool + ":\n" + out, nil
}
