package agent

import (
	"os"
	"strings"
	"testing"
)

func TestToolRegistry_SearchFilesMaxResults(t *testing.T) {
	tmp := t.TempDir()
	orig, _ := os.Getwd()
	os.Chdir(tmp)
	defer os.Chdir(orig)

	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "needle line "+string(rune('0'+i)))
	}
	if err := os.WriteFile("many.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := NewToolRegistry()
	res, err := r.Call("search_files", map[string]any{
		"pattern":     "needle",
		"max_results": 3,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res, "truncated to 3 lines") {
		t.Fatalf("expected truncation: %s", res)
	}
	count := strings.Count(res, "needle")
	if count != 3 {
		t.Fatalf("expected 3 matches in output, got %d: %s", count, res)
	}
}
