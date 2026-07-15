package agent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
