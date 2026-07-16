# Antisthenes

Minimal Go-based AI agent with streaming tool calling, persistent memory, and TUI.

**Current version**: 0.1.5 (injected at build time)

## Requirements

- Go (for building from source)
- **tmux 3.7 or newer** on PATH for persistent terminal tools (`tmux_*`) and the TUI `/tmux` pane
  - el10’s packaged `tmux next-3.4` is **not** supported (broken `capture-pane -p` / corrupt buffers).

## Quick Start

```bash
./antisthenes
```

Run a one-shot query (pipeable):

```bash
./antisthenes --prompt "What is the current time?"
./antisthenes -P "Summarise the last session"
echo "Summarise this" | ./antisthenes -P -
./antisthenes -P @prompt.txt
```

See full command reference: [docs/cli.md](docs/cli.md)

`./antisthenes version` shows the current build version.

## Core Capabilities

- Streaming agent loop with native tool calling and recursion
- SQLite-backed memory with sessions, nudges, and scheduled tasks
- File-based skills (SKILL.md + lazy index)
- Interactive TUI (Bubble Tea; layout and integration per DESIGN-TUI.md)
- Non-interactive / one-shot mode for scripting
- Config-driven multi-endpoint (local llama.cpp, xAI, OpenAI-compatible)
- MCP server/client support (dynamic `tools/list` from registry)
- Gateway abstraction (e.g. Telegram adapter)
- Cron/scheduler integration
- Unified CLI install (`tool_status`, `install_tool`) for rg, nmap, prefcli tools, ansible
- Constrained network scans (`nmap_scan`, config-gated via `nmap_enabled`)
- Policy/approval for sensitive tools (e.g. bash, installs, nmap scans)
- Debug logging and context compression
- Persistent tmux sessions (local + registered SSH hosts) and optional TUI chat-area pane

See [docs/features.md](docs/features.md) for details.

## TUI

Slash commands (`/tools`, `/build`, `/theme`, `/clear`, `/tmux`, and more): [docs/tui.md](docs/tui.md)

## Tools

Core FS/exec + skills, delegation, MCP call, context, Ansible, git, process, installable CLI tools, nmap, tmux, and more.

Full list: [docs/tools.md](docs/tools.md)

## Configuration

Copy `config.example.json` to `config.json` and edit.

See [docs/configuration.md](docs/configuration.md)

Branch: master (v0.1.5)

## SOUL.md

Edit SOUL.md to customize the agent's core system prompt.

## Building

```bash
make build          # native binary ./antisthenes (version from git tag or 0.1.5)
make test           # go test ./...
make release        # static linux/amd64 tarball under dist/
```

Manual equivalent:

```bash
CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=0.1.5" -o antisthenes ./cmd/antisthenes
```

(Version is set in `cmd/antisthenes/main.go` and injected via ldflags.)

### CI / releases

- Push/PR to `master`: `.github/workflows/ci.yml` runs vet, test, and build.
- Tag `v*` (e.g. `git tag v0.1.5 && git push origin v0.1.5`): `.github/workflows/release.yml` builds a static tarball and attaches it to a GitHub/Gitea release (`softprops/action-gh-release@v2`). Gitea also reads `.gitea/workflows/release.yml`.
