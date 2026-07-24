package agent

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nanami/antisthenes/config"
)

const maxHTTPResponseBytes = 1 << 20 // 1 MiB

func registerHTTPTools(r *ToolRegistry) {
	registerHTTPToolsWithUA(r, config.DefaultHTTPUserAgent)
}

// ConfigureHTTPFetch re-registers http_fetch with the given default User-Agent
// (empty → config.DefaultHTTPUserAgent). Call from bootstrap after NewToolRegistry
// so config.http_user_agent is applied on agent and MCP paths.
func ConfigureHTTPFetch(r *ToolRegistry, userAgent string) {
	if r == nil {
		return
	}
	ua := strings.TrimSpace(userAgent)
	if ua == "" {
		ua = config.DefaultHTTPUserAgent
	}
	registerHTTPToolsWithUA(r, ua)
}

func registerHTTPToolsWithUA(r *ToolRegistry, defaultUA string) {
	ua := defaultUA
	r.Register("http_fetch", func(args map[string]any) (string, error) {
		rawURL, ok := args["url"].(string)
		if !ok || strings.TrimSpace(rawURL) == "" {
			return "http_fetch: url is required", nil
		}
		rawURL = strings.TrimSpace(rawURL)

		u, err := url.Parse(rawURL)
		if err != nil {
			return "http_fetch: invalid url: " + err.Error(), nil
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "http_fetch: only http and https URLs are supported", nil
		}

		method := "GET"
		if m, ok := args["method"].(string); ok && strings.TrimSpace(m) != "" {
			method = strings.ToUpper(strings.TrimSpace(m))
		}
		body, _ := args["body"].(string)

		timeoutSec := 30
		switch v := args["timeout"].(type) {
		case float64:
			if int(v) > 0 {
				timeoutSec = int(v)
			}
		case int:
			if v > 0 {
				timeoutSec = v
			}
		}

		var bodyReader io.Reader
		if body != "" {
			bodyReader = strings.NewReader(body)
		}

		req, err := http.NewRequest(method, rawURL, bodyReader)
		if err != nil {
			return "http_fetch: " + err.Error(), nil
		}

		if headers, ok := args["headers"].(map[string]any); ok {
			for k, v := range headers {
				req.Header.Set(k, fmt.Sprint(v))
			}
		}
		// Sites such as Wikipedia reject Go's default "Go-http-client/1.1".
		// Apply configured default only when the caller did not set User-Agent.
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", ua)
		}

		client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "http_fetch: " + err.Error(), nil
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes+1))
		if err != nil {
			return "http_fetch: read body: " + err.Error(), nil
		}
		truncated := len(data) > maxHTTPResponseBytes
		if truncated {
			data = data[:maxHTTPResponseBytes]
		}

		var b strings.Builder
		fmt.Fprintf(&b, "HTTP %d %s\n", resp.StatusCode, resp.Status)
		if ct := resp.Header.Get("Content-Type"); ct != "" {
			fmt.Fprintf(&b, "Content-Type: %s\n", ct)
		}
		b.WriteString("--- body ---\n")
		b.Write(data)
		if truncated {
			b.WriteString("\n... (truncated to 1 MiB)")
		}
		return b.String(), nil
	})
}
