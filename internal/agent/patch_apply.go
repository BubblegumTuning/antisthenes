package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type patchLine struct {
	tag  byte // ' ', '-', '+'
	text string
}

type patchHunk struct {
	oldStart int
	lines    []patchLine
}

// applyPatchDiff applies a unified diff. Uses git apply when available, else a Go fallback.
func applyPatchDiff(diff, pathOverride string) (string, error) {
	diff = strings.TrimSpace(diff)
	if diff == "" {
		return "", fmt.Errorf("empty diff")
	}
	if _, err := exec.LookPath("git"); err == nil {
		if method, err := applyPatchGit(diff); err == nil {
			return method, nil
		}
	}
	return "unified", applyPatchUnified(diff, pathOverride)
}

func applyPatchGit(diff string) (string, error) {
	tmp, err := os.CreateTemp("", "antisthenes-*.patch")
	if err != nil {
		return "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(diff); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "apply", "--whitespace=nowarn", tmpPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git apply: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return "git apply", nil
}

func applyPatchUnified(diff, pathOverride string) error {
	target, hunks, err := parseUnifiedDiff(diff)
	if err != nil {
		return err
	}
	if pathOverride != "" {
		target = pathOverride
	}
	if target == "" {
		return fmt.Errorf("could not determine target file from diff (provide path)")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		return fmt.Errorf("read %s: %w", target, err)
	}
	lines := splitLinesPreserve(data)
	lines, err = applyHunks(lines, hunks)
	if err != nil {
		return err
	}
	content := joinLines(lines)
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return nil
}

func parseUnifiedDiff(diff string) (target string, hunks []patchHunk, err error) {
	var current *patchHunk
	for _, raw := range strings.Split(diff, "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.HasPrefix(line, "+++ ") {
			if target == "" {
				target = normalizeDiffPath(strings.TrimPrefix(line, "+++ "))
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			if current != nil {
				trimTrailingEmptyHunkLines(current)
				hunks = append(hunks, *current)
			}
			oldStart, perr := parseHunkHeader(line)
			if perr != nil {
				return "", nil, perr
			}
			current = &patchHunk{oldStart: oldStart}
			continue
		}
		if current == nil {
			continue
		}
		if line == "" {
			continue
		}
		if line == `\ No newline at end of file` {
			continue
		}
		tag := line[0]
		if tag != ' ' && tag != '-' && tag != '+' {
			continue
		}
		current.lines = append(current.lines, patchLine{tag: tag, text: line[1:]})
	}
	if current != nil {
		trimTrailingEmptyHunkLines(current)
		hunks = append(hunks, *current)
	}
	if len(hunks) == 0 {
		return "", nil, fmt.Errorf("no hunks found in diff")
	}
	return target, hunks, nil
}

func trimTrailingEmptyHunkLines(h *patchHunk) {
	for len(h.lines) > 0 && h.lines[len(h.lines)-1].tag == ' ' && h.lines[len(h.lines)-1].text == "" {
		h.lines = h.lines[:len(h.lines)-1]
	}
}

func normalizeDiffPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimPrefix(p, "b/")
	p = strings.TrimPrefix(p, "a/")
	return filepath.Clean(p)
}

func parseHunkHeader(line string) (oldStart int, err error) {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return 0, fmt.Errorf("invalid hunk header: %s", line)
	}
	oldPart := strings.TrimPrefix(parts[1], "-")
	oldBits := strings.SplitN(oldPart, ",", 2)
	start, err := strconv.Atoi(oldBits[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hunk header: %s", line)
	}
	return start, nil
}

func applyHunks(lines []string, hunks []patchHunk) ([]string, error) {
	cursor := 0
	for _, hunk := range hunks {
		start := findHunkStart(lines, hunk, cursor)
		if start < 0 {
			if hunk.oldStart > 0 && hunk.oldStart-1 < len(lines) {
				start = hunk.oldStart - 1
			} else {
				return nil, fmt.Errorf("hunk context not found")
			}
		}
		var out []string
		out = append(out, lines[:start]...)
		idx := start
		for _, pl := range hunk.lines {
			switch pl.tag {
			case ' ':
				if idx >= len(lines) || lines[idx] != pl.text {
					return nil, fmt.Errorf("context mismatch at line %d", idx+1)
				}
				out = append(out, pl.text)
				idx++
			case '-':
				if idx >= len(lines) || lines[idx] != pl.text {
					return nil, fmt.Errorf("delete mismatch at line %d", idx+1)
				}
				idx++
			case '+':
				out = append(out, pl.text)
			}
		}
		out = append(out, lines[idx:]...)
		lines = out
		cursor = start + 1
	}
	return lines, nil
}

func findHunkStart(lines []string, hunk patchHunk, from int) int {
	pattern := hunkPattern(hunk)
	if len(pattern) == 0 {
		return -1
	}
	for i := from; i <= len(lines)-len(pattern); i++ {
		if matchesPattern(lines[i:i+len(pattern)], pattern) {
			return i
		}
	}
	for i := 0; i <= len(lines)-len(pattern); i++ {
		if matchesPattern(lines[i:i+len(pattern)], pattern) {
			return i
		}
	}
	return -1
}

func hunkPattern(hunk patchHunk) []string {
	var pattern []string
	for _, pl := range hunk.lines {
		if pl.tag == ' ' || pl.tag == '-' {
			pattern = append(pattern, pl.text)
		}
	}
	return pattern
}

func matchesPattern(lines, pattern []string) bool {
	for i := range pattern {
		if lines[i] != pattern[i] {
			return false
		}
	}
	return true
}

func splitLinesPreserve(data []byte) []string {
	s := string(data)
	if s == "" {
		return []string{}
	}
	lines := strings.Split(s, "\n")
	if strings.HasSuffix(s, "\n") {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
