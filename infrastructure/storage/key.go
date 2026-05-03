package storage

import (
	"crypto/md5"
	"fmt"
	"strings"
)

// keyFromURL builds the sharded object key from a URL and file extension.
// Pattern: {hex[0]}/{hex[1]}/{hex[2]}/{hex}.{ext}
func keyFromURL(rawURL, ext string) string {
	hash := md5.Sum([]byte(rawURL))
	hex := fmt.Sprintf("%x", hash)
	ext = sanitizeExt(ext)
	return fmt.Sprintf("%s/%s/%s/%s%s", string(hex[0]), string(hex[1]), string(hex[2]), hex, ext)
}

func sanitizeExt(ext string) string {
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			return r
		}
		return -1
	}, strings.ToLower(ext))
	if len(safe) > 8 {
		safe = safe[:8]
	}
	return safe
}
