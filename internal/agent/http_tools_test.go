package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanami/antisthenes/config"
)

func TestToolRegistry_HTTPFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("X-Test") != "yes" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("pong"))
	}))
	defer srv.Close()

	r := NewToolRegistry()

	res, err := r.Call("http_fetch", map[string]any{"url": srv.URL})
	if err != nil || !strings.Contains(res, "405") {
		t.Fatalf("GET default: %v %s", err, res)
	}

	res, err = r.Call("http_fetch", map[string]any{
		"url":    srv.URL,
		"method": "POST",
		"headers": map[string]any{
			"X-Test": "yes",
		},
	})
	if err != nil || !strings.Contains(res, "200") || !strings.Contains(res, "pong") {
		t.Fatalf("POST: %v %s", err, res)
	}

	res, err = r.Call("http_fetch", map[string]any{"url": "file:///etc/passwd"})
	if err != nil || !strings.Contains(res, "only http and https") {
		t.Fatalf("scheme block: %v %s", err, res)
	}
}

func TestHTTPFetch_DefaultUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	r := NewToolRegistry()
	res, err := r.Call("http_fetch", map[string]any{"url": srv.URL})
	if err != nil || !strings.Contains(res, "200") {
		t.Fatalf("fetch: %v %s", err, res)
	}
	if gotUA != config.DefaultHTTPUserAgent {
		t.Fatalf("default UA: got %q want %q", gotUA, config.DefaultHTTPUserAgent)
	}
	if strings.Contains(gotUA, "Go-http-client") {
		t.Fatalf("must not send Go default UA: %q", gotUA)
	}
}

func TestHTTPFetch_ConfigureUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const custom = "AntisthenesTest/1.0"
	r := NewToolRegistry()
	ConfigureHTTPFetch(r, custom)
	if _, err := r.Call("http_fetch", map[string]any{"url": srv.URL}); err != nil {
		t.Fatal(err)
	}
	if gotUA != custom {
		t.Fatalf("configured UA: got %q want %q", gotUA, custom)
	}

	// Empty configure falls back to default.
	ConfigureHTTPFetch(r, "  ")
	if _, err := r.Call("http_fetch", map[string]any{"url": srv.URL}); err != nil {
		t.Fatal(err)
	}
	if gotUA != config.DefaultHTTPUserAgent {
		t.Fatalf("empty configure → default: got %q", gotUA)
	}
}

func TestHTTPFetch_CallerUserAgentWins(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewToolRegistry()
	ConfigureHTTPFetch(r, "Configured/0")
	_, err := r.Call("http_fetch", map[string]any{
		"url": srv.URL,
		"headers": map[string]any{
			"User-Agent": "PerCall/9",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotUA != "PerCall/9" {
		t.Fatalf("per-call UA should win: got %q", gotUA)
	}
}
