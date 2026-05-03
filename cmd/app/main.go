package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/urfave/cli/v3"

	appCrawler "github.com/atvirokodosprendimai/crawlerv2/application/crawler"
	httpinfra "github.com/atvirokodosprendimai/crawlerv2/infrastructure/http"
	"github.com/atvirokodosprendimai/crawlerv2/infrastructure/persistence"
	"github.com/atvirokodosprendimai/crawlerv2/infrastructure/scheduler"
	"github.com/atvirokodosprendimai/crawlerv2/infrastructure/worker"
)

func main() {
	app := &cli.Command{
		Name:  "crawlerv2",
		Usage: "distributed web crawler",
		Commands: []*cli.Command{
			serverCmd(),
			workerCmd(),
			tokenCmd(),
		},
	}
	if err := app.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

func serverCmd() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "start the master crawl server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "addr",
				Value:   ":8080",
				Sources: cli.EnvVars("HTTP_ADDR"),
			},
			&cli.StringFlag{
				Name:    "db",
				Value:   "crawlerv2.db",
				Sources: cli.EnvVars("DATABASE_PATH"),
			},
			&cli.StringFlag{
				Name:    "bootstrap-token",
				Sources: cli.EnvVars("ADMIN_BOOTSTRAP_TOKEN"),
			},
			&cli.DurationFlag{
				Name:    "stale-timeout",
				Value:   5 * time.Minute,
				Sources: cli.EnvVars("STALE_TASK_TIMEOUT"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			db, err := persistence.Open(cmd.String("db"))
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}

			// Repositories
			domainRepo := persistence.NewDomainRepository(db)
			jobRepo := persistence.NewCrawlJobRepository(db)
			urlRepo := persistence.NewURLRepository(db)
			resultRepo := persistence.NewCrawlResultRepository(db)
			tokenRepo := persistence.NewTokenRepository(db)

			// Bootstrap token
			if tok := cmd.String("bootstrap-token"); tok != "" {
				existing, _ := tokenRepo.FindAll(ctx)
				if len(existing) == 0 {
					manageUC := appCrawler.NewManageTokenUseCase(tokenRepo)
					plain, t, err := manageUC.Create(ctx)
					if err == nil {
						_ = t
						fmt.Printf("bootstrap token created: %s\n", plain)
					}
				}
			}

			// Use cases
			addUC := appCrawler.NewAddDomainUseCase(domainRepo, urlRepo, jobRepo)
			updateUC := appCrawler.NewUpdateDomainUseCase(domainRepo)
			triggerUC := appCrawler.NewTriggerJobUseCase(domainRepo, jobRepo, urlRepo)
			pollUC := appCrawler.NewPollTasksUseCase(jobRepo, domainRepo, urlRepo)
			submitUC := appCrawler.NewSubmitResultsUseCase(jobRepo, domainRepo, urlRepo, resultRepo)
			manageTokenUC := appCrawler.NewManageTokenUseCase(tokenRepo)
			reclaimUC := appCrawler.NewReclaimStaleTasksUseCase(urlRepo)

			// Scheduler
			sched := scheduler.New(triggerUC, reclaimUC, cmd.Duration("stale-timeout"))
			domains, err := domainRepo.FindAll(ctx)
			if err != nil {
				return err
			}
			for i := range domains {
				sched.Register(&domains[i])
			}

			// HTTP handlers
			domainHandlers := httpinfra.NewDomainHandlers(addUC, updateUC, domainRepo)
			domainHandlers.OnDomainSaved = sched.Register

			jobHandlers := httpinfra.NewJobHandlers(triggerUC, jobRepo)
			tokenHandlers := httpinfra.NewTokenHandlers(manageTokenUC)
			workerHandlers := httpinfra.NewWorkerHandlers(pollUC, submitUC)

			router := httpinfra.NewRouter(tokenRepo, domainHandlers, jobHandlers, tokenHandlers, workerHandlers)

			srv := &http.Server{
				Addr:    cmd.String("addr"),
				Handler: router,
			}

			sched.Start(ctx)

			// Graceful shutdown
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-quit
				sched.Stop()
				shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = srv.Shutdown(shutCtx)
			}()

			fmt.Printf("crawlerv2 server listening on %s\n", cmd.String("addr"))
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
}

func workerCmd() *cli.Command {
	return &cli.Command{
		Name:  "worker",
		Usage: "start a worker that polls the master",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "master",
				Required: true,
				Sources:  cli.EnvVars("MASTER_URL"),
			},
			&cli.StringFlag{
				Name:     "token",
				Required: true,
				Sources:  cli.EnvVars("WORKER_TOKEN"),
			},
			&cli.UintFlag{
				Name:    "job-id",
				Required: true,
				Sources: cli.EnvVars("JOB_ID"),
			},
			&cli.IntFlag{
				Name:    "batch",
				Value:   10,
				Sources: cli.EnvVars("WORKER_BATCH"),
			},
			&cli.IntFlag{
				Name:    "concurrency",
				Value:   4,
				Sources: cli.EnvVars("WORKER_CONCURRENCY"),
			},
			&cli.DurationFlag{
				Name:    "poll-interval",
				Value:   5 * time.Second,
				Sources: cli.EnvVars("WORKER_POLL_INTERVAL"),
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			cfg := worker.Config{
				MasterURL:    cmd.String("master"),
				Token:        cmd.String("token"),
				JobID:        cmd.Uint("job-id"),
				BatchSize:    cmd.Int("batch"),
				Concurrency:  cmd.Int("concurrency"),
				PollInterval: cmd.Duration("poll-interval"),
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
			runCtx, cancel := context.WithCancel(ctx)
			go func() {
				<-quit
				cancel()
			}()

			fmt.Printf("worker starting: master=%s job=%d\n", cfg.MasterURL, cfg.JobID)
			worker.NewRunner(cfg).Run(runCtx)
			return nil
		},
	}
}

func tokenCmd() *cli.Command {
	return &cli.Command{
		Name:  "token",
		Usage: "manage bearer tokens",
		Commands: []*cli.Command{
			{
				Name:  "create",
				Usage: "create a new bearer token",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "db",
						Value:   "crawlerv2.db",
						Sources: cli.EnvVars("DATABASE_PATH"),
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					db, err := persistence.Open(cmd.String("db"))
					if err != nil {
						return err
					}
					tokenRepo := persistence.NewTokenRepository(db)
					uc := appCrawler.NewManageTokenUseCase(tokenRepo)
					plain, tok, err := uc.Create(ctx)
					if err != nil {
						return err
					}
					fmt.Printf("token id=%d\n%s\n", tok.ID, plain)
					return nil
				},
			},
			{
				Name:  "revoke",
				Usage: "revoke a bearer token by ID",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "db",
						Value:   "crawlerv2.db",
						Sources: cli.EnvVars("DATABASE_PATH"),
					},
					&cli.UintFlag{
						Name:     "id",
						Required: true,
					},
				},
				Action: func(ctx context.Context, cmd *cli.Command) error {
					db, err := persistence.Open(cmd.String("db"))
					if err != nil {
						return err
					}
					tokenRepo := persistence.NewTokenRepository(db)
					uc := appCrawler.NewManageTokenUseCase(tokenRepo)
					if err := uc.Revoke(ctx, cmd.Uint("id")); err != nil {
						return err
					}
					fmt.Printf("token %d revoked\n", cmd.Uint("id"))
					return nil
				},
			},
		},
	}
}
