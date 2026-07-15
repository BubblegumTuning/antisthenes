# Network Scanning (nmap)

Use this skill when the user asks to discover hosts, check open ports, or verify services on networks they own or are authorized to test.

## Prerequisites

1. `tool_status` with `tool=nmap` — confirm nmap is on PATH
2. If missing: `install_tool` with `tool=nmap` (requires user approval)
3. Ensure `nmap_enabled: true` in `config.json` (default). When false, `nmap_scan` is not available.

## Workflow

```
tool_status({tool: "nmap"})
install_tool({tool: "nmap"})     # only when missing; needs approval
nmap_scan({target, scan_type})   # always needs approval
```

## Scan presets (`nmap_scan`)

| scan_type | Use when |
|-----------|----------|
| `ping` | Host discovery only (`-sn`). **Default — start here.** |
| `quick` | Fast scan of common ports (`-F`) |
| `ports` | TCP connect scan on specific ports (`-sT`, default ports `1-1000`) |
| `service` | Port scan + service version (`-sT -sV`) |

## Rules

- Only scan hosts/networks the user owns or has **explicit permission** to test.
- Prefer `ping` or narrow `ports` lists over wide sweeps.
- One target per call: hostname, IP, or CIDR (e.g. `192.168.1.0/24`).
- Full output is logged under `logs/nmap/`; summarize the tail for the user.
- Do not attempt raw SYN scans or nmap scripts — the tool only supports the presets above.
- If `nmap_scan` is unavailable, say so and offer `tool_status` / `install_tool` steps.

## Example

User: "Is anything alive on 192.168.1.0/24?"

1. `nmap_scan({target: "192.168.1.0/24", scan_type: "ping"})`
2. Summarize hosts that responded; suggest follow-up `ports` scan only if the user asks.