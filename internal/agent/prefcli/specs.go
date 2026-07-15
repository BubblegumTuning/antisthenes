package prefcli

import "strings"

type spec struct {
	bins     []string
	fallback func(args map[string]string) []string
	packages map[Distro]string
}

var specs = map[Tool]spec{
	ToolFd: {
		bins: []string{"fd", "fdfind"},
		fallback: func(a map[string]string) []string {
			name := a["pattern"]
			path := a["path"]
			if path == "" {
				path = "."
			}
			if name == "" {
				return []string{"find", path, "-type", "f"}
			}
			return []string{"find", path, "-type", "f", "-iname", "*" + name + "*"}
		},
		packages: map[Distro]string{
			DistroAlpine: "fd",
			DistroDebian: "fd-find",
			DistroRedHat: "fd",
		},
	},
	ToolBat: {
		bins: []string{"bat"},
		fallback: func(a map[string]string) []string {
			return []string{"cat", a["path"]}
		},
		packages: map[Distro]string{
			DistroAlpine: "bat",
			DistroDebian: "bat",
			DistroRedHat: "bat",
		},
	},
	ToolEza: {
		bins: []string{"eza", "exa"},
		fallback: func(a map[string]string) []string {
			path := a["path"]
			if path == "" {
				path = "."
			}
			if a["long"] == "true" {
				return []string{"ls", "-la", path}
			}
			return []string{"ls", "-1", path}
		},
		packages: map[Distro]string{
			DistroAlpine: "eza",
			DistroDebian: "eza",
			DistroRedHat: "eza",
		},
	},
	ToolFzf: {
		bins: []string{"fzf"},
		fallback: func(a map[string]string) []string {
			query := a["query"]
			if query == "" {
				return []string{"cat"}
			}
			return []string{"grep", "-i", query}
		},
		packages: map[Distro]string{
			DistroAlpine: "fzf",
			DistroDebian: "fzf",
			DistroRedHat: "fzf",
		},
	},
	ToolAstGrep: {
		bins: []string{"ast-grep", "sg"},
		fallback: func(a map[string]string) []string {
			pattern := a["pattern"]
			path := a["path"]
			if path == "" {
				path = "."
			}
			return []string{"grep", "-r", "-n", "-E", pattern, path}
		},
		packages: map[Distro]string{
			DistroAlpine: "ast-grep",
			DistroDebian: "ast-grep",
			DistroRedHat: "ast-grep",
		},
	},
	ToolZoxide: {
		bins: []string{"zoxide"},
		fallback: func(a map[string]string) []string {
			return nil
		},
		packages: map[Distro]string{
			DistroAlpine: "zoxide",
			DistroDebian: "zoxide",
			DistroRedHat: "zoxide",
		},
	},
	ToolDelta: {
		bins: []string{"delta"},
		fallback: func(a map[string]string) []string {
			args := []string{"diff"}
			if a["git"] == "true" {
				args = []string{"git", "diff"}
			}
			if s := a["args"]; s != "" {
				args = append(args, strings.Fields(s)...)
			}
			return args
		},
		packages: map[Distro]string{
			DistroAlpine: "git-delta",
			DistroDebian: "git-delta",
			DistroRedHat: "git-delta",
		},
	},
}
