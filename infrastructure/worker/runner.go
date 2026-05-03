package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Config holds worker runtime configuration.
type Config struct {
	MasterURL    string
	Token        string
	DomainID     uint // optional — 0 means work on any running job
	BatchSize    int
	Concurrency  int
	PollInterval time.Duration
	// StorageConfig path — loaded into a Registry at startup. If empty, uses local ./files.
	StorageConfigPath string
	// DefaultFilesDir is used when StorageConfigPath is empty.
	DefaultFilesDir string
}

// TaskResponse mirrors the server's task JSON.
type TaskResponse struct {
	TaskID  uint   `json:"task_id"`
	URL     string `json:"url"`
	JobID   uint   `json:"job_id"`
	Depth   int    `json:"depth"`
	Extract struct {
		ExtractTitle   bool   `json:"extract_title"`
		ExtractMeta    bool   `json:"extract_meta"`
		ExtractBody    bool   `json:"extract_body"`
		DownloadBinary bool   `json:"download_binary"`
		FilesDir       string `json:"files_dir"`
		MaxFileSizeMB  int    `json:"max_file_size_mb"`
	} `json:"extract"`
	Politeness struct {
		RespectRobotsTxt        bool `json:"respect_robots_txt"`
		CrawlDelayMs            int  `json:"crawl_delay_ms"`
		MaxConcurrencyPerWorker int  `json:"max_concurrency_per_worker"`
	} `json:"politeness"`
}

type resultItem struct {
	TaskID       uint     `json:"task_id"`
	URL          string   `json:"url"`
	Depth        int      `json:"depth"`
	StatusCode   int      `json:"status_code"`
	ResponseTime int64    `json:"response_time_ms"`
	ContentType  string   `json:"content_type"`
	Title        string   `json:"title"`
	MetaDesc     string   `json:"meta_desc"`
	Body         string   `json:"body"`
	Links        []string `json:"links"`
	FilePath     string   `json:"file_path,omitempty"`
	FileSize     int64    `json:"file_size,omitempty"`
	FileHash     string   `json:"file_hash,omitempty"`
	Error        string   `json:"error,omitempty"`
}

// Runner is the worker poll loop.
type Runner struct {
	cfg    Config
	client *http.Client
	robots *RobotsCache
}

func NewRunner(cfg Config) *Runner {
	client := &http.Client{Timeout: 30 * time.Second}
	return &Runner{
		cfg:    cfg,
		client: client,
		robots: NewRobotsCache(client, 10*time.Minute),
	}
}

// Run starts the polling loop and blocks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) {
	concurrency := r.cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 4
	}
	sem := make(chan struct{}, concurrency)

	backoff := r.cfg.PollInterval
	if backoff <= 0 {
		backoff = 5 * time.Second
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		tasks, err := r.poll(ctx)
		if err != nil {
			log.Printf("worker: poll error: %v", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		if len(tasks) == 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			continue
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		var results []resultItem

		for _, task := range tasks {
			wg.Add(1)
			sem <- struct{}{}
			go func(t TaskResponse) {
				defer wg.Done()
				defer func() { <-sem }()

				if t.Politeness.CrawlDelayMs > 0 {
					time.Sleep(time.Duration(t.Politeness.CrawlDelayMs) * time.Millisecond)
				}

				res := r.crawl(ctx, t)
				mu.Lock()
				results = append(results, res)
				mu.Unlock()
			}(task)
		}
		wg.Wait()

		// All tasks in a batch share the same job ID (PollTasksUseCase returns from one job at a time)
		jobID := tasks[0].JobID
		if err := r.submit(ctx, jobID, results); err != nil {
			log.Printf("worker: submit error: %v", err)
		}
	}
}

func (r *Runner) poll(ctx context.Context) ([]TaskResponse, error) {
	u := fmt.Sprintf("%s/worker/tasks/next?batch=%d", r.cfg.MasterURL, r.cfg.BatchSize)
	if r.cfg.DomainID > 0 {
		u += fmt.Sprintf("&domain_id=%d", r.cfg.DomainID)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+r.cfg.Token)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("poll: status %d", resp.StatusCode)
	}

	var tasks []TaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *Runner) crawl(ctx context.Context, t TaskResponse) resultItem {
	res := resultItem{
		TaskID: t.TaskID,
		URL:    t.URL,
		Depth:  t.Depth,
	}

	// robots.txt check
	if t.Politeness.RespectRobotsTxt {
		u, err := url.Parse(t.URL)
		if err == nil && !r.robots.IsAllowed(u.Scheme, u.Host, u.Path) {
			res.Error = "disallowed by robots.txt"
			return res
		}
	}

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	req.Header.Set("User-Agent", "crawlerv2-bot/1.0")

	resp, err := r.client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer resp.Body.Close()

	res.StatusCode = resp.StatusCode
	res.ResponseTime = time.Since(start).Milliseconds()
	res.ContentType = resp.Header.Get("Content-Type")

	if resp.StatusCode != http.StatusOK {
		return res
	}

	filesDir := t.Extract.FilesDir
	if filesDir == "" {
		filesDir = "files"
	}

	maxBytes := int64(5 * 1024 * 1024) // 5 MB default for HTML
	if t.Extract.MaxFileSizeMB > 0 {
		maxBytes = int64(t.Extract.MaxFileSizeMB) * 1024 * 1024
	}

	ext := ExtFromContentType(res.ContentType)

	if IsHTML(res.ContentType) {
		// Read body (limited), parse links + metadata, save to filesystem
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
		if err != nil {
			res.Error = err.Error()
			return res
		}

		page := ParsePage(bytes.NewReader(body), t.URL, t.Extract.ExtractBody)
		if t.Extract.ExtractTitle {
			res.Title = page.Title
		}
		if t.Extract.ExtractMeta {
			res.MetaDesc = page.MetaDesc
		}
		if t.Extract.ExtractBody {
			res.Body = page.Body
		}
		res.Links = page.Links

		// Save HTML to filesystem
		fs := NewFileStore(filesDir)
		relPath, size, hash, err := fs.Write(t.URL, ext, bytes.NewReader(body))
		if err != nil {
			log.Printf("filestore: HTML save failed for %s: %v", t.URL, err)
		} else {
			res.FilePath = relPath
			res.FileSize = size
			res.FileHash = hash
		}

	} else if t.Extract.DownloadBinary {
		// Stream directly to filesystem — no in-memory buffering
		fs := NewFileStore(filesDir)
		relPath, size, hash, err := fs.Write(t.URL, ext, io.LimitReader(resp.Body, maxBytes))
		if err != nil {
			log.Printf("filestore: binary save failed for %s: %v", t.URL, err)
			res.Error = err.Error()
		} else {
			res.FilePath = relPath
			res.FileSize = size
			res.FileHash = hash
		}
	}

	return res
}

func (r *Runner) submit(ctx context.Context, jobID uint, results []resultItem) error {
	if len(results) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"results": results})
	u := fmt.Sprintf("%s/worker/jobs/%d/results", r.cfg.MasterURL, jobID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("submit: status %d", resp.StatusCode)
	}
	return nil
}
