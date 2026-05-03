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
	JobID        uint
	BatchSize    int
	Concurrency  int
	PollInterval time.Duration
}

// TaskResponse mirrors the server's task JSON.
type TaskResponse struct {
	TaskID  uint   `json:"task_id"`
	URL     string `json:"url"`
	JobID   uint   `json:"job_id"`
	Depth   int    `json:"depth"`
	Extract struct {
		ExtractTitle bool `json:"extract_title"`
		ExtractMeta  bool `json:"extract_meta"`
		ExtractBody  bool `json:"extract_body"`
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

		if err := r.submit(ctx, results); err != nil {
			log.Printf("worker: submit error: %v", err)
		}
	}
}

func (r *Runner) poll(ctx context.Context) ([]TaskResponse, error) {
	u := fmt.Sprintf("%s/worker/tasks/next?job_id=%d&batch=%d",
		r.cfg.MasterURL, r.cfg.JobID, r.cfg.BatchSize)
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

	if resp.StatusCode == http.StatusOK {
		page := ParsePage(io.LimitReader(resp.Body, 5*1024*1024), t.URL, t.Extract.ExtractBody)
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
	}
	return res
}

func (r *Runner) submit(ctx context.Context, results []resultItem) error {
	if len(results) == 0 {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"results": results})
	u := fmt.Sprintf("%s/worker/jobs/%d/results", r.cfg.MasterURL, r.cfg.JobID)
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
