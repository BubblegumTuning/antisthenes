package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// registerFSTools registers the filesystem-related tools (list_dir, create_dir, read_file, write_file).
// These were previously inlined inside NewToolRegistry.
func registerFSTools(r *ToolRegistry) {
	r.Register("list_dir", func(args map[string]any) (string, error) {
		path := "."
		if p, ok := args["path"].(string); ok && p != "" {
			path = p
		}
		_, out, err := listDirPreferred(path)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(out), nil
	})

	r.Register("create_dir", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return "create_dir: path is required", nil
		}
		path = strings.TrimSpace(path)
		if err := validateRelativePath(path); err != nil {
			return "create_dir: " + err.Error(), nil
		}
		cmd := "mkdir " + path
		if ok, denied := r.resolveApproval("create_dir", cmd); !ok {
			if denied {
				return "create_dir: denied by user", nil
			}
			return "create_dir: Approval required. Use approve_tool.", nil
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return "", err
		}
		return "Directory created: " + path, nil
	})

	r.Register("read_file", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return "read_file: path is required", nil
		}
		encoding := parseEncodingArg(args["encoding"])
		maxBytes := parseMaxBytesArg(args["max_bytes"])

		if encoding == "base64" {
			data, err := readFileLimited(path, maxBytes)
			if err != nil {
				if os.IsNotExist(err) {
					return "read_file: not found: " + path, nil
				}
				return "", err
			}
			return formatBase64Read(path, data), nil
		}

		if encoding != "text" {
			return "read_file: unsupported encoding (use text or base64)", nil
		}

		_, content, err := readFilePreferred(path)
		if err != nil {
			if os.IsNotExist(err) {
				return "read_file: not found: " + path, nil
			}
			return "", err
		}
		if isLikelyBinary([]byte(content)) {
			return "read_file: binary file detected; re-read with encoding=base64", nil
		}
		return content, nil
	})

	r.Register("write_file", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return "write_file: path is required", nil
		}
		content, _ := args["content"].(string)
		encoding := parseEncodingArg(args["encoding"])
		data, err := decodeWriteContent(content, encoding)
		if err != nil {
			return "write_file: " + err.Error(), nil
		}

		perm := os.FileMode(0644)
		if modeStr, ok := args["mode"].(string); ok && strings.TrimSpace(modeStr) != "" {
			perm, err = parseFileMode(strings.TrimSpace(modeStr))
			if err != nil {
				return "write_file: " + err.Error(), nil
			}
		}

		// Auto-create parent directories (per create_dir plan item)
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", err
			}
		}
		if err := writeFileBytes(path, data, perm); err != nil {
			return "", err
		}
		if encoding == "base64" {
			return fmt.Sprintf("File written: %s (%d bytes, encoding=base64)", path, len(data)), nil
		}
		return "File written: " + path, nil
	})

	r.Register("delete_file", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || path == "" {
			return "delete_file: path is required", nil
		}
		path = strings.TrimSpace(path)
		if err := validateRelativePath(path); err != nil {
			return "delete_file: " + err.Error(), nil
		}
		recursive, _ := args["recursive"].(bool)
		cmd := "rm " + path
		if recursive {
			cmd = "rm -rf " + path
		}
		if ok, denied := r.resolveApproval("delete_file", cmd); !ok {
			if denied {
				return "delete_file: denied by user", nil
			}
			return "delete_file: approval required. Use approve_tool or TUI popup.", nil
		}
		var err error
		if recursive {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil {
			if os.IsNotExist(err) {
				return "delete_file: not found: " + path, nil
			}
			return "", err
		}
		return "Deleted: " + path, nil
	})

	r.Register("move_file", func(args map[string]any) (string, error) {
		src, ok := args["src"].(string)
		if !ok || strings.TrimSpace(src) == "" {
			return "move_file: src is required", nil
		}
		dst, ok := args["dst"].(string)
		if !ok || strings.TrimSpace(dst) == "" {
			return "move_file: dst is required", nil
		}
		src = strings.TrimSpace(src)
		dst = strings.TrimSpace(dst)
		if err := validateRelativePath(src); err != nil {
			return "move_file: src: " + err.Error(), nil
		}
		if err := validateRelativePath(dst); err != nil {
			return "move_file: dst: " + err.Error(), nil
		}
		cmd := "mv " + src + " " + dst
		if ok, denied := r.resolveApproval("move_file", cmd); !ok {
			if denied {
				return "move_file: denied by user", nil
			}
			return "move_file: approval required. Use approve_tool or TUI popup.", nil
		}
		if err := os.Rename(src, dst); err != nil {
			if os.IsNotExist(err) {
				return "move_file: source not found: " + src, nil
			}
			return "", err
		}
		return "Moved " + src + " -> " + dst, nil
	})

	r.Register("file_stat", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || strings.TrimSpace(path) == "" {
			return "file_stat: path is required", nil
		}
		path = strings.TrimSpace(path)
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return "file_stat: not found: " + path, nil
			}
			return "", err
		}
		mode := info.Mode()
		var kind string
		switch {
		case mode.IsDir():
			kind = "directory"
		case mode&os.ModeSymlink != 0:
			kind = "symlink"
		default:
			kind = "file"
		}
		return fmt.Sprintf(
			"file_stat: %s\n  type: %s\n  size: %d bytes\n  mode: %s (%04o)\n  modified: %s",
			path, kind, info.Size(), mode.String(), mode.Perm(), info.ModTime().Format(time.RFC3339),
		), nil
	})

	r.Register("chmod", func(args map[string]any) (string, error) {
		path, ok := args["path"].(string)
		if !ok || strings.TrimSpace(path) == "" {
			return "chmod: path is required", nil
		}
		modeStr, ok := args["mode"].(string)
		if !ok || strings.TrimSpace(modeStr) == "" {
			return "chmod: mode is required (octal, e.g. 755 or 0644)", nil
		}
		path = strings.TrimSpace(path)
		modeStr = strings.TrimSpace(modeStr)
		perm, err := parseFileMode(modeStr)
		if err != nil {
			return "chmod: " + err.Error(), nil
		}
		cmd := "chmod " + modeStr + " " + path
		if ok, denied := r.resolveApproval("chmod", cmd); !ok {
			if denied {
				return "chmod: denied by user", nil
			}
			return "chmod: approval required. Use approve_tool or TUI popup.", nil
		}
		if err := os.Chmod(path, perm); err != nil {
			if os.IsNotExist(err) {
				return "chmod: not found: " + path, nil
			}
			return "", err
		}
		return fmt.Sprintf("chmod: set %s on %s", modeStr, path), nil
	})

	r.Register("copy_file", func(args map[string]any) (string, error) {
		src, ok := args["src"].(string)
		if !ok || strings.TrimSpace(src) == "" {
			return "copy_file: src is required", nil
		}
		dst, ok := args["dst"].(string)
		if !ok || strings.TrimSpace(dst) == "" {
			return "copy_file: dst is required", nil
		}
		src = strings.TrimSpace(src)
		dst = strings.TrimSpace(dst)
		if err := validateRelativePath(src); err != nil {
			return "copy_file: src: " + err.Error(), nil
		}
		if err := validateRelativePath(dst); err != nil {
			return "copy_file: dst: " + err.Error(), nil
		}
		if err := copyFilePath(src, dst); err != nil {
			if os.IsNotExist(err) {
				return "copy_file: source not found: " + src, nil
			}
			return "", err
		}
		return "Copied " + src + " -> " + dst, nil
	})
}

func parseFileMode(modeStr string) (os.FileMode, error) {
	modeStr = strings.TrimPrefix(modeStr, "0")
	if modeStr == "" {
		return 0, fmt.Errorf("invalid mode")
	}
	val, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid octal mode %q", modeStr)
	}
	return os.FileMode(val), nil
}

func validateRelativePath(path string) error {
	clean := filepath.Clean(strings.TrimSpace(path))
	if clean == "." || clean == ".." {
		return fmt.Errorf("invalid or unsafe path")
	}
	if strings.Contains(clean, "..") || filepath.IsAbs(clean) {
		return fmt.Errorf("invalid or unsafe path")
	}
	return nil
}

func copyFilePath(src, dst string) error {
	if dir := filepath.Dir(dst); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
