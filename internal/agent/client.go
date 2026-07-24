package agent

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
)

const streamOpenRetries = 3

func newOpenAIClient(apiKey, baseURL string) *openai.Client {
	cfg := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		cfg.BaseURL = baseURL
	}
	cfg.HTTPClient = &http.Client{
		// No overall timeout: streamed completions can run for a long time.
		Timeout: 0,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 3 * time.Minute,
		},
	}
	return openai.NewClientWithConfig(cfg)
}

// isTransientNetErr reports whether an error is likely recoverable with retry
// or a non-streaming fallback (connection reset, broken pipe, timeouts, etc.).
func isTransientNetErr(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	s := strings.ToLower(err.Error())
	for _, frag := range []string{
		"connection reset",
		"broken pipe",
		"write tcp",
		"read tcp",
		"timeout",
		"tls handshake timeout",
		"i/o timeout",
		"unexpected eof",
		"connection refused",
		"no such host",
	} {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}

func (l *Loop) openStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	var last error
	for attempt := 0; attempt < streamOpenRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		stream, err := l.client.CreateChatCompletionStream(ctx, req)
		if err == nil {
			return stream, nil
		}
		last = err
		// Some local servers reject stream_options; drop and retry once per attempt.
		if req.StreamOptions != nil && streamOptionsUnsupported(err) {
			req.StreamOptions = nil
			stream, err = l.client.CreateChatCompletionStream(ctx, req)
			if err == nil {
				return stream, nil
			}
			last = err
		}
		if !isTransientNetErr(err) {
			return nil, err
		}
	}
	return nil, last
}

func streamOptionsUnsupported(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, frag := range []string{
		"stream_options",
		"include_usage",
		"unknown field",
		"unrecognized request argument",
	} {
		if strings.Contains(s, frag) {
			return true
		}
	}
	// HTTP 400 from picky OpenAI-compatible servers
	return strings.Contains(s, "status code: 400") || strings.Contains(s, "statuscode=400")
}
