package crawler

import (
	"net/url"
	"regexp"
	"strings"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type URLStatus string

const (
	URLStatusPending    URLStatus = "pending"
	URLStatusInProgress URLStatus = "in_progress"
	URLStatusDone       URLStatus = "done"
	URLStatusFailed     URLStatus = "failed"
)

type ScopeConfig struct {
	IncludeSubdomains bool
	PathFilter        string // regex; empty = allow all
	MaxDepth          int    // 0 = unlimited
	MaxPagesPerRun    int    // 0 = unlimited
}

type PolitenessConfig struct {
	RespectRobotsTxt       bool
	CrawlDelayMs           int
	MaxConcurrencyPerWorker int
	MaxConcurrencyGlobal    int // 0 = unlimited
}

type ExtractConfig struct {
	ExtractTitle       bool
	ExtractMeta        bool
	ExtractBody        bool
}

// RootDomain extracts the registrable root domain from a host.
// sub.example.com → example.com
func RootDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	// strip port
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		host = host[:idx]
	}
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// NormalizeURL returns a canonical form of rawURL or an error.
func NormalizeURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Fragment = ""
	u.RawQuery = u.Query().Encode() // sort query params
	result := u.String()
	result = strings.TrimRight(result, "/")
	return result, nil
}

// URLInScope reports whether targetURL should be enqueued given domain scope.
func URLInScope(targetURL string, seedHost string, cfg ScopeConfig, depth int) bool {
	u, err := url.Parse(targetURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}

	host := strings.ToLower(u.Hostname())
	seed := strings.ToLower(seedHost)

	if cfg.IncludeSubdomains {
		root := RootDomain(seed)
		if host != seed && !strings.HasSuffix(host, "."+root) {
			return false
		}
	} else {
		if host != seed {
			return false
		}
	}

	if cfg.MaxDepth > 0 && depth > cfg.MaxDepth {
		return false
	}

	if cfg.PathFilter != "" {
		matched, err := regexp.MatchString(cfg.PathFilter, u.Path)
		if err != nil || !matched {
			return false
		}
	}

	return true
}
