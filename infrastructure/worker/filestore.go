package worker

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileStore writes downloaded resources to a sharded directory tree.
// Path: {base}/{md5[0]}/{md5[1]}/{md5[2]}/{md5}.{ext}
type FileStore struct {
	Base string
}

func NewFileStore(base string) *FileStore {
	return &FileStore{Base: base}
}

// Write streams r to disk, returns (relPath, fileSize, hexMD5, error).
// relPath is relative to Base.
func (fs *FileStore) Write(rawURL string, ext string, r io.Reader) (relPath string, size int64, hexHash string, err error) {
	hash := md5.Sum([]byte(rawURL))
	hex := fmt.Sprintf("%x", hash)

	dir := filepath.Join(fs.Base, string(hex[0]), string(hex[1]), string(hex[2]))
	if err = os.MkdirAll(dir, 0755); err != nil {
		return
	}

	ext = sanitizeExt(ext)
	filename := hex + ext
	fullPath := filepath.Join(dir, filename)

	f, err := os.Create(fullPath)
	if err != nil {
		return
	}
	defer f.Close()

	// Tee through MD5 hasher while writing
	hasher := md5.New()
	w := io.MultiWriter(f, hasher)
	size, err = io.Copy(w, r)
	if err != nil {
		return
	}

	hexHash = fmt.Sprintf("%x", hasher.Sum(nil))
	relPath = filepath.Join(string(hex[0]), string(hex[1]), string(hex[2]), filename)
	return
}

// Path returns the full path for a given relPath.
func (fs *FileStore) Path(relPath string) string {
	return filepath.Join(fs.Base, relPath)
}

func sanitizeExt(ext string) string {
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	// allow only safe characters
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

// ExtFromContentType returns a file extension for a MIME content-type.
func ExtFromContentType(ct string) string {
	ct = strings.ToLower(strings.Split(ct, ";")[0])
	ct = strings.TrimSpace(ct)
	switch ct {
	case "text/html":
		return ".html"
	case "application/xhtml+xml":
		return ".xhtml"
	case "image/svg+xml":
		return ".svg"
	case "application/pdf":
		return ".pdf"
	case "application/msword":
		return ".doc"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.ms-powerpoint":
		return ".ppt"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "application/zip":
		return ".zip"
	case "application/x-tar", "application/gzip":
		return ".tar"
	case "text/plain":
		return ".txt"
	case "text/csv":
		return ".csv"
	case "application/json":
		return ".json"
	case "application/xml", "text/xml":
		return ".xml"
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico"
	}
	return ""
}

// IsHTML reports whether a content-type is HTML.
func IsHTML(ct string) bool {
	ct = strings.ToLower(strings.Split(ct, ";")[0])
	ct = strings.TrimSpace(ct)
	return ct == "text/html" || ct == "application/xhtml+xml"
}
