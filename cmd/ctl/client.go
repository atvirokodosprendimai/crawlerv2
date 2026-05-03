package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	base   string
	token  string
	http   *http.Client
}

func NewClient(base, token string) *Client {
	return &Client{
		base:  base,
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.base+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		if e.Error != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, e.Error)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

func (c *Client) get(path string, out any) error    { return c.do("GET", path, nil, out) }
func (c *Client) post(path string, body, out any) error { return c.do("POST", path, body, out) }
func (c *Client) put(path string, body, out any) error  { return c.do("PUT", path, body, out) }
func (c *Client) del(path string) error             { return c.do("DELETE", path, nil, nil) }

// --- API types (mirrors server DTOs) ---

type Domain struct {
	ID        uint      `json:"id"`
	Host      string    `json:"host"`
	Scope     Scope     `json:"scope"`
	Politeness Politeness `json:"politeness"`
	Cron      string    `json:"cron"`
	CreatedAt time.Time `json:"created_at"`
}

type Scope struct {
	IncludeSubdomains bool   `json:"include_subdomains"`
	PathFilter        string `json:"path_filter"`
	MaxDepth          int    `json:"max_depth"`
	MaxPagesPerRun    int    `json:"max_pages_per_run"`
}

type Politeness struct {
	RespectRobotsTxt        bool `json:"respect_robots_txt"`
	CrawlDelayMs            int  `json:"crawl_delay_ms"`
	MaxConcurrencyPerWorker int  `json:"max_concurrency_per_worker"`
	MaxConcurrencyGlobal    int  `json:"max_concurrency_global"`
}

type Extract struct {
	ExtractTitle bool `json:"extract_title"`
	ExtractMeta  bool `json:"extract_meta"`
	ExtractBody  bool `json:"extract_body"`
}

type Job struct {
	ID           uint       `json:"id"`
	DomainID     uint       `json:"domain_id"`
	Status       string     `json:"status"`
	PagesCrawled int        `json:"pages_crawled"`
	StartedAt    *time.Time `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Token struct {
	ID        uint      `json:"id"`
	Revoked   bool      `json:"revoked"`
	CreatedAt time.Time `json:"created_at"`
}

type TokenCreated struct {
	Token
	Token_ string `json:"token"`
}
