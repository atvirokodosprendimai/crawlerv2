package worker

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type robotsEntry struct {
	disallowed []string
	fetchedAt  time.Time
}

// RobotsCache fetches and caches robots.txt per host.
type RobotsCache struct {
	mu      sync.RWMutex
	cache   map[string]*robotsEntry
	ttl     time.Duration
	client  *http.Client
}

func NewRobotsCache(client *http.Client, ttl time.Duration) *RobotsCache {
	return &RobotsCache{
		cache:  make(map[string]*robotsEntry),
		ttl:    ttl,
		client: client,
	}
}

// IsAllowed returns true if the given URL path is allowed for crawling.
func (rc *RobotsCache) IsAllowed(scheme, host, path string) bool {
	rc.mu.RLock()
	entry, ok := rc.cache[host]
	rc.mu.RUnlock()

	if !ok || time.Since(entry.fetchedAt) > rc.ttl {
		entry = rc.fetch(scheme, host)
		rc.mu.Lock()
		rc.cache[host] = entry
		rc.mu.Unlock()
	}

	for _, d := range entry.disallowed {
		if d == "/" || strings.HasPrefix(path, d) {
			return false
		}
	}
	return true
}

func (rc *RobotsCache) fetch(scheme, host string) *robotsEntry {
	entry := &robotsEntry{fetchedAt: time.Now()}
	url := fmt.Sprintf("%s://%s/robots.txt", scheme, host)
	resp, err := rc.client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return entry
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	entry.disallowed = parseRobots(string(body))
	return entry
}

// parseRobots extracts Disallow rules for * agent.
func parseRobots(content string) []string {
	var disallowed []string
	relevant := false
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "user-agent:") {
			agent := strings.TrimSpace(strings.TrimPrefix(lower, "user-agent:"))
			relevant = agent == "*"
			continue
		}
		if relevant && strings.HasPrefix(lower, "disallow:") {
			path := strings.TrimSpace(strings.TrimPrefix(line, strings.Split(line, ":")[0]+":"))
			if path != "" {
				disallowed = append(disallowed, path)
			}
		}
	}
	return disallowed
}
