---
tldr: Full implementation of crawlerv2 — Go DDD master/worker web crawler with SQLite, urfave/cli v3, REST polling
status: completed
---

# Plan: Implement crawlerv2 distributed web crawler

## Context

- Spec: [[spec - crawlerv2 - distributed web crawler with master-worker architecture]]

## Phases

### Phase 1 - Project scaffold - status: completed

1. [x] Init go.mod, add all dependencies
   - `github.com/urfave/cli/v3`, `gorm.io/gorm`, `github.com/glebarez/sqlite`, `github.com/go-chi/chi/v5`, `github.com/robfig/cron/v3`, `golang.org/x/net/html`, `golang.org/x/crypto`
2. [x] Create directory structure
   - `cmd/app/`, `domain/crawler/`, `application/crawler/`, `infrastructure/persistence/`, `infrastructure/http/`, `infrastructure/scheduler/`, `infrastructure/worker/`
3. [x] Wire `cmd/app/main.go` with urfave/cli v3 — `server`, `worker`, `token create`, `token revoke` subcommands

### Phase 2 - Domain layer - status: completed

1. [x] Value objects: `JobStatus`, `URLStatus`, `ScopeConfig`, `PolitenessConfig`, `ExtractConfig`
   - => `URLInScope` handles subdomains, path regex, max depth
   - => `RootDomain` extracts registrable root from hostname
   - => `NormalizeURL` canonicalizes URLs
2. [x] Entities: `CrawlDomain`, `CrawlJob`, `URLRecord`, `CrawlResult`, `Token`, `OutboundLink`
3. [x] Repository interfaces: `DomainRepository`, `CrawlJobRepository`, `URLRepository`, `CrawlResultRepository`, `TokenRepository`

### Phase 3 - Persistence - status: completed

1. [x] GORM models with composite uniqueIndex on `(domain_id, raw_url)` for dedup
2. [x] DB init: WAL mode, `_busy_timeout=5000` for multi-process access, `max_open_conns=1`, AutoMigrate
3. [x] `GormDomainRepository` — upsert via `clause.OnConflict{UpdateAll:true}`
4. [x] `GormCrawlJobRepository` — FindActive checks pending+running statuses
5. [x] `GormURLRepository`
   - => `ClaimBatch` uses transaction: SELECT then UPDATE to in_progress
   - => `ReclaimStale` uses `updated_at < threshold`
   - => `Enqueue` uses `OnConflict{DoNothing:true}` on (domain_id, raw_url)
   - => `RootDomain` receives hostname (not full URL) — fixed via `parseHost`
6. [x] `GormCrawlResultRepository`
7. [x] `GormTokenRepository` — bcrypt hash stored; lookup via `FindAll` + `CompareHashAndPassword`

### Phase 4 - Application services - status: completed

1. [x] `AddDomainUseCase` — saves domain
2. [x] `UpdateDomainUseCase` — updates scope/politeness/cron
3. [x] `TriggerJobUseCase` — checks active job, creates job, seeds root URL into queue
4. [x] `PollTasksUseCase` — enforces global concurrency cap via `CountInProgress(rootDomain)`, returns `TaskDTO` slice
5. [x] `SubmitResultsUseCase` — persists results, extracts+filters links via `URLInScope`, enqueues, increments pages
6. [x] `ReclaimStaleTasksUseCase` — delegates to `URLRepository.ReclaimStale`
7. [x] `ManageTokenUseCase` — generates 32-byte random token, bcrypt hash, returns plaintext once

### Phase 5 - HTTP server - status: completed

1. [x] chi router with `middleware.Logger` + `middleware.Recoverer`
2. [x] `BearerAuthMiddleware` — extracts Bearer token, iterates active tokens, bcrypt compare
3. [x] Domain handlers: POST/GET/PUT/DELETE `/api/domains`, `/api/domains/:id`
   - => `OnDomainSaved` callback wires live cron re-registration
4. [x] Job handlers: POST `/api/domains/:id/jobs`, GET `/api/jobs`, GET `/api/jobs/:id`
5. [x] Token handlers: POST/GET `/api/tokens`, DELETE `/api/tokens/:id`
6. [x] Worker handlers: GET `/worker/tasks/next` (204 on empty), POST `/worker/jobs/:jobID/results`

### Phase 6 - Scheduler - status: completed

1. [x] `robfig/cron` wrapper with per-domain entry registry
2. [x] Server startup loads all domains and registers their cron expressions
3. [x] `OnDomainSaved` hook: live register/update without server restart
4. [x] Stale reclaim ticker every 1 minute via `time.Ticker`

### Phase 7 - Worker logic - status: completed

1. [x] `WorkerRunner` with configurable poll interval and backoff on 204/error
2. [x] Semaphore-based per-worker concurrency limit
3. [x] Per-task: `net/http` fetch, `crawl_delay_ms` respected, 5MB body limit
4. [x] HTML parser: `golang.org/x/net/html` — title, meta description, all `<a href>` links
   - => `resolveURL` handles relative, absolute, protocol-relative URLs
5. [x] `RobotsCache`: fetches+parses robots.txt, 10-min TTL, `*` agent only
6. [x] Batch result submit to `POST /worker/jobs/:jobID/results`
7. [x] Graceful shutdown via context cancellation

### Phase 8 - CLI wiring - status: completed

1. [x] `server` subcommand — all flags with `cli.EnvVars(...)` fallback
2. [x] `worker` subcommand — master, token, job-id, batch, concurrency, poll-interval
3. [x] `token create` — opens DB, creates token, prints plaintext
4. [x] `token revoke` — opens DB, revokes by ID
5. [x] All flags backed by env vars

## Verification

Verified via smoke tests:
- [x] `crawlerv2 server` starts, serves `/health` 200
- [x] `crawlerv2 token create` outputs usable plaintext token
- [x] POST `/api/domains` → 201, domain stored
- [x] POST `/api/domains/1/jobs` → 201, job running, seed URL enqueued
- [x] GET `/worker/tasks/next?job_id=1` → 200, returns seed URL task
- [x] Invalid token → 401

## Progress Log

- 2026-05-03 10:49 — All 8 phases implemented. Binary builds clean. Smoke tests pass.
  - Fixed: composite uniqueIndex on url_records(domain_id, raw_url)
  - Fixed: RootDomain received full URL instead of hostname in url_repo
  - Fixed: SQLite busy timeout for multi-process DB access (`_busy_timeout=5000`)
