package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/nanami/antisthenes/internal/agent"
)

func TestStartBridge_Notify(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "1",
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "notified reply"}},
			},
		})
	}
	ts := httptest.NewServer(http.HandlerFunc(handler))
	defer ts.Close()

	loop := agent.NewLoop("sk-dummy", "test-model", ts.URL)
	g := NewGateway(nil)
	fa := &fakeAdapter{name: "notify", incoming: make(chan MessageEvent, 1)}
	g.RegisterAdapter(fa)

	var mu sync.Mutex
	var notes []string
	opts := BridgeOptions{
		Notify: func(text string) { mu.Lock(); notes = append(notes, text); mu.Unlock() },
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := g.StartBridge(ctx, loop, opts); err != nil {
		t.Fatal(err)
	}

	fa.incoming <- MessageEvent{Platform: "notify", ChatID: "1", UserID: "u1", Text: "hello bridge"}
	close(fa.incoming)

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(notes)
		mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected notifications, got %v", notes)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(notes) < 2 {
		t.Fatalf("expected at least 2 notifications, got %v", notes)
	}
	if notes[0] == "" || notes[len(notes)-1] == "" {
		t.Fatalf("empty notification: %v", notes)
	}
}
