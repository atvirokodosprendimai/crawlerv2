---
tldr: Distributed Go web crawler — public REST master orchestrates workers via bearer-auth polling; DDD architecture; SQLite persistence via GORM without CGO
---

# crawlerv2

Distributed web crawler/spider. One master service (public internet) coordinates N stateless workers. Workers poll master for URL batches, crawl them, and submit results. New URLs discovered during crawl are queued for subsequent runs.

## Target

Enable scalable, distributed web crawling where anyone with a bearer token can spin up a worker node and contribute capacity to crawl jobs defined on the master. Domains and crawl scope are configured centrally; workers are interchangeable and stateless.

## Behaviour

### Domains & Scope

- Master holds a list of seed domains to crawl
- Each domain carries scope config:
  - `include_subdomains` flag — crawl `*.example.com` when true
  - `path_filter` regex — only enqueue URLs matching pattern (e.g. `/blog/.*`)
  - `max_depth` — discard URLs more than N hops from seed
  - `max_pages_per_run` — stop enqueuing after N pages crawled in one job run
- Domain can be added/removed via API

### Crawl Jobs

- Job = one crawl run over a domain
- Triggered two ways:
  - **On-demand** — `POST /api/domains/:id/jobs`
  - **Scheduled** — cron expression per domain; master scheduler enqueues job automatically
- Job tracks: status (pending / running / completed / failed), start/end time, pages crawled count
- Only one active job per domain at a time; new trigger while running is rejected or queued

### URL Queue

- URLs discovered during crawl are normalized and stored with state: `pending | in_progress | done | failed`
- Deduplication by normalized URL within a domain
- `in_progress` URLs that exceed a stale timeout (e.g. 5 min) revert to `pending`
- `done` URLs re-queued after configurable TTL for re-crawl in subsequent job runs
- URLs outside domain scope (subdomain flag, path filter, max depth) are discarded, not stored

### Worker Protocol

- Workers are stateless — no registration required, just a valid bearer token
- Worker polls: `GET /worker/tasks/next?batch=N` — receives batch of URL tasks
- Each task contains: task ID, URL, job ID, depth, extract config
- Worker crawls URLs, collects results, submits: `POST /worker/tasks/results`
- Worker sends heartbeat or result within timeout; else master reclaims tasks
- Master marks tasks `done` or `failed` based on result payload

### Crawl Results (Configurable per Job)

Per-job extract config controls what worker collects and returns:

| Field | Default |
|---|---|
| HTTP status code | always |
| Response time (ms) | always |
| Content-Type | always |
| Page title | optional |
| Meta description | optional |
| Full HTML body | optional |
| Outbound links | always (for URL discovery) |

### Politeness (Configurable per Job)

- `respect_robots_txt` bool — parse and honor `robots.txt` per domain
- `crawl_delay_ms` — minimum delay between requests per worker per domain
- `max_concurrency_per_worker` — max parallel requests a single worker may open to this domain
- `max_concurrency_global` — hard cap on total in-flight requests across **all workers** for a given `*.domain.tld`
  - Master enforces this: task batch size returned to a worker is reduced when global in-flight count approaches cap
  - In-flight count = number of tasks in `in_progress` state for that domain
  - Cap applies per effective root domain — `a.example.com` and `b.example.com` share the same `example.com` cap
- Workers apply per-worker settings locally; global cap is enforced server-side at poll time

### Auth

- Bearer tokens stored in DB (`tokens` table)
- Token created/revoked via admin API (separate admin token or local-only endpoint)
- All `/worker/*` and `/api/*` endpoints require valid bearer token
- Token carries no permissions beyond valid/invalid — all token holders have full access

## Design

### DDD Layers

```
crawlerv2/
  cmd/
    app/          ← single binary: master + embedded scheduler
                     CLI entry point via urfave/cli v3
  domain/
    crawler/      ← CrawlDomain, CrawlJob, URL, CrawlResult, Token aggregates
                     repository interfaces, domain services, value objects
  application/
    crawler/      ← use cases: AddDomain, TriggerJob, PollTasks, SubmitResults, DiscoverURLs
  infrastructure/
    persistence/  ← GORM repositories (SQLite, modernc.org/sqlite — no CGO)
    http/         ← chi router, handlers, middleware (auth, logging)
    scheduler/    ← cron-based job trigger (robfig/cron or stdlib time)
```

### CLI (urfave/cli v3)

Binary exposes subcommands via `github.com/urfave/cli/v3`:

```
crawlerv2 server   --addr :8080 --db crawlerv2.db --bootstrap-token <tok>
crawlerv2 worker   --master https://host:8080 --token <tok> --batch 10 --concurrency 4
crawlerv2 token    create --db crawlerv2.db
crawlerv2 token    revoke --db crawlerv2.db --id <id>
```

| Subcommand | Flags |
|---|---|
| `server` | `--addr`, `--db`, `--bootstrap-token`, `--stale-timeout` |
| `worker` | `--master`, `--token`, `--batch`, `--concurrency`, `--poll-interval` |
| `token create` | `--db` |
| `token revoke` | `--db`, `--id` |

- Flags fall back to env vars (urfave/cli v3 `Sources: cli.EnvVars(...)`)
- Env var names match existing `Configuration` table (e.g. `HTTP_ADDR`, `DATABASE_PATH`)

### Aggregates

- **CrawlDomain** — root: Domain + ScopeConfig + cron schedule
- **CrawlJob** — root: Job metadata + status + stats; child: CrawlTask (batch assignment)
- **URLRecord** — root: normalized URL + state machine + depth + last crawled
- **CrawlResult** — root: result payload linked to URLRecord + Job
- **Token** — root: token string + created/revoked state

### URL State Machine

```
pending → in_progress → done
                     ↘ failed → pending (retry up to max_retries)
in_progress (stale timeout) → pending
done (re-crawl TTL elapsed) → pending
```

### Key Interfaces (domain layer)

```go
type URLRepository interface {
    ClaimBatch(ctx, jobID, batchSize int) ([]URLRecord, error)
    MarkDone(ctx, taskID, result CrawlResult) error
    MarkFailed(ctx, taskID, reason string) error
    Enqueue(ctx, urls []URLRecord) error
    ReclaimStale(ctx, threshold time.Duration) (int, error)
}

type CrawlJobRepository interface {
    Create(ctx, job CrawlJob) error
    UpdateStatus(ctx, jobID int, status JobStatus) error
    FindActive(ctx, domainID int) (*CrawlJob, error)
}

type DomainRepository interface {
    Save(ctx, domain CrawlDomain) error
    FindAll(ctx) ([]CrawlDomain, error)
    FindByID(ctx, id int) (*CrawlDomain, error)
}
```

### SQLite / GORM Setup

- Driver: `gorm.io/driver/sqlite` backed by `modernc.org/sqlite` (pure Go, no CGO)
  - Use `github.com/glebarez/sqlite` as GORM adapter for modernc
- Migrations via `db.AutoMigrate(...)` at startup
- WAL mode enabled for concurrent readers (workers + scheduler)
- Single DB file; path configurable via env

### HTTP API

**Admin / API (bearer token required)**

| Method | Path | Description |
|---|---|---|
| POST | /api/domains | Add domain + scope config |
| GET | /api/domains | List domains |
| PUT | /api/domains/:id | Update scope/schedule |
| DELETE | /api/domains/:id | Remove domain |
| POST | /api/domains/:id/jobs | Trigger manual crawl |
| GET | /api/jobs | List jobs (filter by domain, status) |
| GET | /api/jobs/:id | Job detail + stats |
| POST | /api/tokens | Create bearer token |
| DELETE | /api/tokens/:id | Revoke token |

**Worker endpoints (bearer token required)**

| Method | Path | Description |
|---|---|---|
| GET | /worker/tasks/next | Poll for URL batch |
| POST | /worker/tasks/results | Submit crawl results |

### Configuration (env vars)

| Var | Description |
|---|---|
| `DATABASE_PATH` | SQLite file path (default: `crawlerv2.db`) |
| `HTTP_ADDR` | Listen address (default: `:8080`) |
| `STALE_TASK_TIMEOUT` | Duration before in_progress task reclaimed (default: `5m`) |
| `ADMIN_BOOTSTRAP_TOKEN` | First token seeded at startup if no tokens exist |

## Verification

- Worker can poll empty queue and get 204 with no tasks
- Worker submits results → URLs marked done → outbound links enqueued as pending
- Scope filter: URL outside path_filter regex never enqueued
- Subdomain off: `sub.example.com` URL from `example.com` crawl is discarded
- Max depth: URL at depth > max_depth is discarded
- Global concurrency cap: when in-flight task count for `example.com` = `max_concurrency_global`, poll returns 0 tasks for any subdomain of `example.com` until tasks complete
- Stale reclaim: task held > STALE_TASK_TIMEOUT reverts to pending on next poll
- Scheduled trigger fires at cron interval; skips if job already running
- Invalid/missing bearer token → 401 on all protected routes
- Revoked token → 401 immediately

## Friction

- SQLite WAL handles read concurrency but write contention increases with many workers — consider `max_open_conns=1` on writer to serialize writes
- `modernc.org/sqlite` (via `glebarez/sqlite`) is slower than CGO sqlite; acceptable for this use case
- Workers are anonymous (no identity) — can't track which worker crawled what; tradeoff for simplicity
- Single binary design means master + scheduler coupled; separate worker binary lives outside this repo

## Interactions

- Workers are separate deployments that depend on this master's HTTP API
- No external message queue — polling is the coordination mechanism

## Future

{[!] Worker binary — standalone Go binary that polls this master}
{[?] Result webhooks — POST crawl results to external URL on completion}
{[?] Duplicate content detection — hash page body, skip storing identical re-crawls}
{[?] Sitemap.xml seeding — seed URL queue from sitemap on job start}
{[?] Prometheus metrics endpoint — expose crawl throughput, queue depth, error rate}
