package installable

import "github.com/nanami/antisthenes/internal/agent/prefcli"

// Strategy describes how install_tool should install an entry.
type Strategy int

const (
	StrategyPkgMgr Strategy = iota
	StrategyVenvPip
	StrategyManualOnly
)

// Entry is a CLI dependency the agent may install or check.
type Entry struct {
	ID         string
	Aliases    []string
	Bins       []string
	Packages   map[prefcli.Distro]string
	Strategy   Strategy
	VenvDir    string
	PipPackage string
	ManualHint string
}

var catalog = []Entry{
	{
		ID: "rg", Aliases: []string{"ripgrep"},
		Bins: []string{"rg"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "ripgrep", prefcli.DistroDebian: "ripgrep", prefcli.DistroRedHat: "ripgrep",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "fd", Aliases: []string{"fdfind"},
		Bins: []string{"fd", "fdfind"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "fd", prefcli.DistroDebian: "fd-find", prefcli.DistroRedHat: "fd",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "bat", Bins: []string{"bat"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "bat", prefcli.DistroDebian: "bat", prefcli.DistroRedHat: "bat",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "eza", Aliases: []string{"exa"},
		Bins: []string{"eza", "exa"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "eza", prefcli.DistroDebian: "eza", prefcli.DistroRedHat: "eza",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "fzf", Bins: []string{"fzf"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "fzf", prefcli.DistroDebian: "fzf", prefcli.DistroRedHat: "fzf",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "ast-grep", Aliases: []string{"ast_grep", "sg"},
		Bins: []string{"ast-grep", "sg"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "ast-grep", prefcli.DistroDebian: "ast-grep", prefcli.DistroRedHat: "ast-grep",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "zoxide", Bins: []string{"zoxide"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "zoxide", prefcli.DistroDebian: "zoxide", prefcli.DistroRedHat: "zoxide",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "delta", Aliases: []string{"git-delta"},
		Bins: []string{"delta"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "git-delta", prefcli.DistroDebian: "git-delta", prefcli.DistroRedHat: "git-delta",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID: "nmap", Bins: []string{"nmap"},
		Packages: map[prefcli.Distro]string{
			prefcli.DistroAlpine: "nmap", prefcli.DistroDebian: "nmap", prefcli.DistroRedHat: "nmap",
		},
		Strategy: StrategyPkgMgr,
	},
	{
		ID:       "ansible",
		Bins:     []string{"ansible", "ansible-playbook"},
		Strategy: StrategyVenvPip, VenvDir: ".ansible-venv", PipPackage: "ansible",
	},
	{
		ID: "goban-cli", Aliases: []string{"goban"},
		Bins:       []string{"goban-cli"},
		Strategy:   StrategyManualOnly,
		ManualHint: "goban-cli is not in standard distro repos. Install from upstream releases or build from source, then ensure goban-cli is on PATH.",
	},
}
