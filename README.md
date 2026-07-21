# Antisthenes

Minimal Go-based AI agent with streaming tool calling, persistent memory, and TUI.

**Version**: 0.3.2 (injected at build time)

## Requirements

- Go (to build from source)
- **tmux 3.7 or newer** on `PATH` for persistent terminal tools (`tmux_*`) and the optional TUI `/tmux` pane  
  Older builds (including some distro packages based on tmux 3.4) are not supported for pane capture.

## Quick start

```bash
# Build
make build
# or: go build -o antisthenes ./cmd/antisthenes

# Interactive TUI
./antisthenes

# One-shot (scriptable)
./antisthenes --prompt "What is the current time?"
./antisthenes -P "Summarise the last session"
echo "Summarise this" | ./antisthenes -P -
./antisthenes -P @prompt.txt
```

`./antisthenes version` prints the build version.

Full CLI reference: [docs/cli.md](docs/cli.md)

## Capabilities

- Streaming agent loop with native tool calling
- SQLite-backed memory (sessions with titles, nudges, scheduled tasks)
- File-based skills (`SKILL.md` + lazy index)
- Interactive Bubble Tea TUI and non-interactive one-shot mode
- Multi-endpoint configuration plus optional `aux_models` for cheap/async work (e.g. session titles)
- MCP server/client
- Optional gateway adapters (e.g. Telegram)
- Optional cron/scheduler (`cron_enabled` in config; off by default in the TUI)
- Policy/approval for sensitive tools
- Install helpers for common CLI tools; optional constrained `nmap_scan`
- Persistent tmux sessions (local and registered SSH hosts)
- `/iterative` multi-phase Plan → Execute → Review builds (async; optional supervised mode)

Details: [docs/features.md](docs/features.md)

## Documentation

| Doc | Contents |
|-----|----------|
| [docs/cli.md](docs/cli.md) | CLI flags and subcommands |
| [docs/configuration.md](docs/configuration.md) | `config.json` reference |
| [docs/features.md](docs/features.md) | Feature overview |
| [docs/tools.md](docs/tools.md) | Agent tools |
| [docs/tui.md](docs/tui.md) | TUI slash commands and keys |

## Configuration

```bash
cp config.example.json config.json
# edit endpoints, model names, and options
```

See [docs/configuration.md](docs/configuration.md)

## Building
See [docs/configuration.md](docs/configuration.md). `config.json` is gitignored; do not commit secrets.

## TUI

Slash commands include `/help`, `/tools`, `/iterative`, `/build`, `/theme`, `/tmux`, `/mouse`, `/copy`, and others.  
Reference: [docs/tui.md](docs/tui.md)

### `/iterative` (summary)

1. Start with `/iterative` and a project name.  
2. Choose supervised mode when prompted (`y/N`, default **N**).  
3. Plan in conversation; confirm when ready.  
4. **Unsupervised**: async Plan → Execute → Review cycles until done, failed, cancelled, or the execute cap.  
5. **Supervised**: Plan first, then choose an executor before Execute → Review.  
6. While a job runs: `cancel` or **Ctrl+C once** interrupts that window’s job; Ctrl+C again quits the app.  
7. Other chat windows may run their own jobs in parallel.

## Development

```bash
make build          # ./antisthenes
make test           # go test ./...
make release        # static linux/amd64 tarball under dist/
```

Manual build:

```bash
CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=0.3.2" -o antisthenes ./cmd/antisthenes
```

Suggested checks before merge:

```bash
gofmt -l -s .
go vet ./...
go test ./...
go build ./...
```

### CI / releases

- Push/PR to `master`: CI runs vet, test, and build (`.github/workflows/ci.yml`).
- Tag `v*` : release workflow builds a static tarball and attaches it to the release. Gitea may use `.gitea/workflows/release.yml`.

Release archives contain the binary plus `README.md`, `config.example.json`, and `SOUL.md`.

## SOUL.md

Edit [SOUL.md](SOUL.md) to customize the agent’s core system prompt.
