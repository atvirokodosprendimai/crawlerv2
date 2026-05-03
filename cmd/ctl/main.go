package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/urfave/cli/v3"
)

func main() {
	cfg := loadConfig()

	app := &cli.Command{
		Name:  "crawlerctl",
		Usage: "CLI client for crawlerv2 API",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "server",
				Aliases: []string{"s"},
				Value:   cfg.Server,
				Usage:   "master server URL",
				Sources: cli.EnvVars("CRAWLERV2_SERVER"),
			},
			&cli.StringFlag{
				Name:    "token",
				Aliases: []string{"t"},
				Value:   cfg.Token,
				Usage:   "bearer token",
				Sources: cli.EnvVars("CRAWLERV2_TOKEN"),
			},
			&cli.BoolFlag{
				Name:    "json",
				Aliases: []string{"j"},
				Usage:   "output raw JSON",
			},
		},
		Commands: []*cli.Command{
			configCmd(),
			domainCmd(),
			jobCmd(),
			tokenCmd(),
		},
	}

	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func makeClient(cmd *cli.Command) (*Client, error) {
	jsonOutput = cmd.Root().Bool("json")
	server := cmd.Root().String("server")
	token := cmd.Root().String("token")
	if server == "" {
		return nil, fmt.Errorf("--server required (or run: crawlerctl config set-server <url>)")
	}
	if token == "" {
		return nil, fmt.Errorf("--token required (or run: crawlerctl config set-token <token>)")
	}
	return NewClient(server, token), nil
}

// --- config command ---

func configCmd() *cli.Command {
	return &cli.Command{
		Name:  "config",
		Usage: "manage local config (~/.crawlerv2.json)",
		Commands: []*cli.Command{
			{
				Name:      "set-server",
				Usage:     "set default server URL",
				ArgsUsage: "<url>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					url := cmd.Args().First()
					if url == "" {
						return fmt.Errorf("url required")
					}
					cfg := loadConfig()
					cfg.Server = url
					if err := saveConfig(cfg); err != nil {
						return err
					}
					fmt.Printf("server set to %s\n", url)
					return nil
				},
			},
			{
				Name:      "set-token",
				Usage:     "set default bearer token",
				ArgsUsage: "<token>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					tok := cmd.Args().First()
					if tok == "" {
						return fmt.Errorf("token required")
					}
					cfg := loadConfig()
					cfg.Token = tok
					if err := saveConfig(cfg); err != nil {
						return err
					}
					fmt.Println("token saved")
					return nil
				},
			},
			{
				Name:  "show",
				Usage: "show current config",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					cfg := loadConfig()
					fmt.Printf("server: %s\n", orDash(cfg.Server))
					if cfg.Token != "" {
						fmt.Printf("token:  %s...%s\n", cfg.Token[:4], cfg.Token[len(cfg.Token)-4:])
					} else {
						fmt.Printf("token:  -\n")
					}
					fmt.Printf("file:   %s\n", configPath())
					return nil
				},
			},
		},
	}
}

// --- domains command ---

func domainCmd() *cli.Command {
	return &cli.Command{
		Name:    "domains",
		Aliases: []string{"d"},
		Usage:   "manage crawl domains",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list all domains",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					var domains []Domain
					if err := c.get("/api/domains", &domains); err != nil {
						return err
					}
					printDomains(domains)
					return nil
				},
			},
			{
				Name:      "get",
				Usage:     "show domain detail",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					id := cmd.Args().First()
					if id == "" {
						return fmt.Errorf("id required")
					}
					var d Domain
					if err := c.get("/api/domains/"+id, &d); err != nil {
						return err
					}
					printDomain(d)
					return nil
				},
			},
			{
				Name:      "add",
				Usage:     "add a domain to crawl",
				ArgsUsage: "<host>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "subdomains", Usage: "include subdomains"},
					&cli.StringFlag{Name: "path-filter", Usage: "path regex filter"},
					&cli.IntFlag{Name: "max-depth", Usage: "max crawl depth (0=unlimited)"},
					&cli.IntFlag{Name: "max-pages", Usage: "max pages per run (0=unlimited)"},
					&cli.BoolFlag{Name: "robots", Usage: "respect robots.txt"},
					&cli.IntFlag{Name: "delay", Usage: "crawl delay ms"},
					&cli.IntFlag{Name: "conc-worker", Usage: "max concurrency per worker"},
					&cli.IntFlag{Name: "conc-global", Usage: "max global concurrency for *.domain.tld"},
					&cli.StringFlag{Name: "cron", Usage: "cron schedule (e.g. '0 2 * * *')"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					host := cmd.Args().First()
					if host == "" {
						return fmt.Errorf("host required")
					}
					body := map[string]any{
						"host": host,
						"scope": map[string]any{
							"include_subdomains": cmd.Bool("subdomains"),
							"path_filter":        cmd.String("path-filter"),
							"max_depth":          cmd.Int("max-depth"),
							"max_pages_per_run":  cmd.Int("max-pages"),
						},
						"politeness": map[string]any{
							"respect_robots_txt":         cmd.Bool("robots"),
							"crawl_delay_ms":             cmd.Int("delay"),
							"max_concurrency_per_worker": cmd.Int("conc-worker"),
							"max_concurrency_global":     cmd.Int("conc-global"),
						},
						"cron": cmd.String("cron"),
					}
					var d Domain
					if err := c.post("/api/domains", body, &d); err != nil {
						return err
					}
					fmt.Printf("domain added: id=%d host=%s\n", d.ID, d.Host)
					return nil
				},
			},
			{
				Name:      "update",
				Usage:     "update domain config",
				ArgsUsage: "<id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "host"},
					&cli.BoolFlag{Name: "subdomains"},
					&cli.StringFlag{Name: "path-filter"},
					&cli.IntFlag{Name: "max-depth"},
					&cli.IntFlag{Name: "max-pages"},
					&cli.BoolFlag{Name: "robots"},
					&cli.IntFlag{Name: "delay"},
					&cli.IntFlag{Name: "conc-worker"},
					&cli.IntFlag{Name: "conc-global"},
					&cli.StringFlag{Name: "cron"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					id := cmd.Args().First()
					if id == "" {
						return fmt.Errorf("id required")
					}
					// fetch current then merge
					var current Domain
					if err := c.get("/api/domains/"+id, &current); err != nil {
						return err
					}
					if cmd.IsSet("host") {
						current.Host = cmd.String("host")
					}
					if cmd.IsSet("subdomains") {
						current.Scope.IncludeSubdomains = cmd.Bool("subdomains")
					}
					if cmd.IsSet("path-filter") {
						current.Scope.PathFilter = cmd.String("path-filter")
					}
					if cmd.IsSet("max-depth") {
						current.Scope.MaxDepth = cmd.Int("max-depth")
					}
					if cmd.IsSet("max-pages") {
						current.Scope.MaxPagesPerRun = cmd.Int("max-pages")
					}
					if cmd.IsSet("robots") {
						current.Politeness.RespectRobotsTxt = cmd.Bool("robots")
					}
					if cmd.IsSet("delay") {
						current.Politeness.CrawlDelayMs = cmd.Int("delay")
					}
					if cmd.IsSet("conc-worker") {
						current.Politeness.MaxConcurrencyPerWorker = cmd.Int("conc-worker")
					}
					if cmd.IsSet("conc-global") {
						current.Politeness.MaxConcurrencyGlobal = cmd.Int("conc-global")
					}
					if cmd.IsSet("cron") {
						current.Cron = cmd.String("cron")
					}
					body := map[string]any{
						"host":       current.Host,
						"scope":      current.Scope,
						"politeness": current.Politeness,
						"cron":       current.Cron,
					}
					var d Domain
					if err := c.put("/api/domains/"+id, body, &d); err != nil {
						return err
					}
					fmt.Printf("domain %s updated\n", id)
					return nil
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a domain",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					id := cmd.Args().First()
					if id == "" {
						return fmt.Errorf("id required")
					}
					if err := c.del("/api/domains/" + id); err != nil {
						return err
					}
					fmt.Printf("domain %s deleted\n", id)
					return nil
				},
			},
		},
	}
}

// --- jobs command ---

func jobCmd() *cli.Command {
	return &cli.Command{
		Name:    "jobs",
		Aliases: []string{"j"},
		Usage:   "manage crawl jobs",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list jobs",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "domain-id", Aliases: []string{"d"}, Usage: "filter by domain id"},
					&cli.StringFlag{Name: "status", Aliases: []string{"s"}, Usage: "filter by status (pending|running|completed|failed)"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					path := "/api/jobs"
					sep := "?"
					if v := cmd.String("domain-id"); v != "" {
						path += sep + "domain_id=" + v
						sep = "&"
					}
					if v := cmd.String("status"); v != "" {
						path += sep + "status=" + v
					}
					var jobs []Job
					if err := c.get(path, &jobs); err != nil {
						return err
					}
					printJobs(jobs)
					return nil
				},
			},
			{
				Name:      "get",
				Usage:     "show job detail",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					id := cmd.Args().First()
					if id == "" {
						return fmt.Errorf("id required")
					}
					var j Job
					if err := c.get("/api/jobs/"+id, &j); err != nil {
						return err
					}
					printJob(j)
					return nil
				},
			},
			{
				Name:      "trigger",
				Usage:     "trigger a crawl job for a domain",
				ArgsUsage: "<domain-id>",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "title", Usage: "extract page title"},
					&cli.BoolFlag{Name: "meta", Usage: "extract meta description"},
					&cli.BoolFlag{Name: "body", Usage: "extract full HTML body"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					id := cmd.Args().First()
					if id == "" {
						return fmt.Errorf("domain-id required")
					}
					body := map[string]any{
						"extract": map[string]any{
							"extract_title": cmd.Bool("title"),
							"extract_meta":  cmd.Bool("meta"),
							"extract_body":  cmd.Bool("body"),
						},
					}
					var j Job
					if err := c.post("/api/domains/"+id+"/jobs", body, &j); err != nil {
						return err
					}
					fmt.Printf("job started: id=%d domain=%d status=%s\n", j.ID, j.DomainID, j.Status)
					return nil
				},
			},
		},
	}
}

// --- tokens command ---

func tokenCmd() *cli.Command {
	return &cli.Command{
		Name:    "tokens",
		Aliases: []string{"tok"},
		Usage:   "manage bearer tokens",
		Commands: []*cli.Command{
			{
				Name:  "list",
				Usage: "list all tokens",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					var tokens []Token
					if err := c.get("/api/tokens", &tokens); err != nil {
						return err
					}
					printTokens(tokens)
					return nil
				},
			},
			{
				Name:  "create",
				Usage: "create a new bearer token",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "save", Usage: "save new token to ~/.crawlerv2.json"},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					var t TokenCreated
					if err := c.post("/api/tokens", map[string]any{}, &t); err != nil {
						return err
					}
					fmt.Printf("id:    %d\ntoken: %s\n", t.ID, t.Token_)
					if cmd.Bool("save") {
						cfg := loadConfig()
						cfg.Token = t.Token_
						if err := saveConfig(cfg); err != nil {
							return fmt.Errorf("token created but config save failed: %w", err)
						}
						fmt.Println("saved to ~/.crawlerv2.json")
					}
					return nil
				},
			},
			{
				Name:      "revoke",
				Usage:     "revoke a token by ID",
				ArgsUsage: "<id>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					c, err := makeClient(cmd)
					if err != nil {
						return err
					}
					id := cmd.Args().First()
					if id == "" {
						return fmt.Errorf("id required")
					}
					if err := c.del("/api/tokens/" + id); err != nil {
						return err
					}
					fmt.Printf("token %s revoked\n", id)
					return nil
				},
			},
		},
	}
}
