package agent

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

const (
	defaultMaxBinaryRead = 1 << 20 // 1 MiB
	hardMaxBinaryRead    = 8 << 20 // 8 MiB
)

func parseEncodingArg(v any) string {
	enc, _ := v.(string)
	enc = strings.ToLower(strings.TrimSpace(enc))
	if enc == "" {
		return "text"
	}
	return enc
}

func parseMaxBytesArg(v any) int {
	maxBytes := defaultMaxBinaryRead
	switch n := v.(type) {
	case float64:
		maxBytes = int(n)
	case int:
		maxBytes = n
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxBinaryRead
	}
	if maxBytes > hardMaxBinaryRead {
		maxBytes = hardMaxBinaryRead
	}
	return maxBytes
}

func readFileLimited(path string, maxBytes int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, int64(maxBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("file exceeds max read size (%d bytes)", maxBytes)
	}
	return data, nil
}

func isLikelyBinary(data []byte) bool {
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	if len(data) > 0 && !utf8.Valid(data) {
		return true
	}
	return false
}

func formatBase64Read(path string, data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("read_file: %s (%d bytes, encoding=base64)\n%s", path, len(data), encoded)
}

func decodeWriteContent(content, encoding string) ([]byte, error) {
	switch encoding {
	case "text":
		return []byte(content), nil
	case "base64":
		content = strings.TrimSpace(content)
		if content == "" {
			return nil, nil
		}
		return base64.StdEncoding.DecodeString(content)
	default:
		return nil, fmt.Errorf("unsupported encoding %q (use text or base64)", encoding)
	}
}

func writeFileBytes(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
