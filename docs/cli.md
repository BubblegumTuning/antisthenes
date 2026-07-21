# CLI Reference

**Updated**: 2026-07-21 / v0.3.2

## Interactive Mode

```bash
./antisthenes
```

Launches the full TUI.

## One-shot Mode (Recommended for Scripting)

```bash
./antisthenes --prompt "your question here"
./antisthenes -P "your question here"
./antisthenes -P -                              # read prompt from stdin
./antisthenes -P @prompt.txt                    # read prompt from file
./antisthenes --prompt-file prompt.txt          # read prompt from file
```

Outputs only the final assistant response to stdout. Ideal for piping and automation.

Examples:
```bash
./antisthenes -P "What is 2+2?"
echo "Summarise this log" | ./antisthenes -P -
./antisthenes -P @prompt.txt
./antisthenes --prompt-file prompt.txt
```

## Help

```bash
./antisthenes --help
./antisthenes -h
```

## Subcommands

| Command     | Description                              |
|-------------|------------------------------------------|
| `version`   | Print version string                     |
| `index`     | Regenerate `skills/index.json`           |
| `config`    | Display current configuration            |
| `sessions`  | List recent sessions                     |
| `mcp`       | Start MCP server on stdio (dynamic `tools/list` from registry) |
| `model`     | Interactive model/endpoint configuration |

## Flags

- `--prompt`, `-P` — Run a single prompt and exit with clean output (`-` for stdin, `@file` for file)
- `--prompt-file` — Run a single prompt read from a file
- `--help`, `-h` — Show usage information

All other arguments fall back to the interactive TUI.

## MCP Server (`mcp` subcommand)

```bash
./antisthenes mcp
```

Starts a minimal MCP server on stdio (JSON-RPC 2.0). `tools/list` introspects the tool registry dynamically — each entry includes `name`, `description`, and `inputSchema` (JSON Schema parameters, or an empty object schema when none is defined). `tools/call` executes any registered tool via the same registry (policy and side effects preserved).

The standalone MCP server uses the base `NewToolRegistry()` set plus config-gated nmap/network tools (~57–58 tools by default). It does **not** include `mcp_call`, `mcp_list_tools`, cron scheduling tools, or aux-model tools; those are added only on the TUI and one-shot agent paths (to avoid recursive MCP, missing scheduler lifecycle, and missing LLM wiring). See [docs/tools.md](tools.md) for registration notes.

Startup banners and errors go to **stderr** so stdout stays JSON-RPC-only.

## TUI Slash Commands

Interactive mode supports slash commands in the edit box. Full reference: [docs/tui.md](tui.md).