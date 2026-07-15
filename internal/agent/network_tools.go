package agent

import "strings"

// RegisterNetworkTools adds network_status when enabled is true.
func RegisterNetworkTools(r *ToolRegistry, enabled bool) {
	if !enabled {
		return
	}
	r.Register("network_status", func(args map[string]any) (string, error) {
		opts := parseNetworkStatusArgs(args)
		snap := collectNetworkStatus(opts)
		enrichNetworkStatus(&snap, opts.Detail)
		return formatNetworkStatus(snap), nil
	})
}

func parseNetworkStatusArgs(args map[string]any) networkStatusOptions {
	opts := networkStatusOptions{Detail: "brief"}
	if iface, ok := args["interface"].(string); ok {
		opts.Interface = strings.TrimSpace(iface)
	}
	if include, ok := args["include_loopback"].(bool); ok {
		opts.IncludeLoopback = include
	}
	if detail, ok := args["detail"].(string); ok {
		d := strings.TrimSpace(strings.ToLower(detail))
		if d == "full" {
			opts.Detail = "full"
		}
	}
	return opts
}