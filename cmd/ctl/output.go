package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

var jsonOutput bool

func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func newTable(headers ...string) *tabwriter.Writer {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	fmt.Fprintln(w, strings.Repeat("-\t", len(headers)))
	return w
}

func fmtTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

func fmtBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func fmtInt(n int) string {
	if n == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", n)
}

func printDomains(domains []Domain) {
	if jsonOutput {
		printJSON(domains)
		return
	}
	w := newTable("ID", "HOST", "SUBDOMS", "DEPTH", "PAGES/RUN", "DELAY(ms)", "CONC.GLOBAL", "CRON")
	for _, d := range domains {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			d.ID,
			d.Host,
			fmtBool(d.Scope.IncludeSubdomains),
			fmtInt(d.Scope.MaxDepth),
			fmtInt(d.Scope.MaxPagesPerRun),
			fmtInt(d.Politeness.CrawlDelayMs),
			fmtInt(d.Politeness.MaxConcurrencyGlobal),
			orDash(d.Cron),
		)
	}
	w.Flush()
}

func printDomain(d Domain) {
	if jsonOutput {
		printJSON(d)
		return
	}
	fmt.Printf("ID:               %d\n", d.ID)
	fmt.Printf("Host:             %s\n", d.Host)
	fmt.Printf("Include subdoms:  %s\n", fmtBool(d.Scope.IncludeSubdomains))
	fmt.Printf("Path filter:      %s\n", orDash(d.Scope.PathFilter))
	fmt.Printf("Max depth:        %s\n", fmtInt(d.Scope.MaxDepth))
	fmt.Printf("Max pages/run:    %s\n", fmtInt(d.Scope.MaxPagesPerRun))
	fmt.Printf("Robots.txt:       %s\n", fmtBool(d.Politeness.RespectRobotsTxt))
	fmt.Printf("Crawl delay (ms): %s\n", fmtInt(d.Politeness.CrawlDelayMs))
	fmt.Printf("Conc/worker:      %s\n", fmtInt(d.Politeness.MaxConcurrencyPerWorker))
	fmt.Printf("Conc/global:      %s\n", fmtInt(d.Politeness.MaxConcurrencyGlobal))
	fmt.Printf("Cron:             %s\n", orDash(d.Cron))
	fmt.Printf("Created:          %s\n", fmtTime(&d.CreatedAt))
}

func printJobs(jobs []Job) {
	if jsonOutput {
		printJSON(jobs)
		return
	}
	w := newTable("ID", "DOMAIN", "STATUS", "PAGES", "STARTED", "ENDED")
	for _, j := range jobs {
		fmt.Fprintf(w, "%d\t%d\t%s\t%d\t%s\t%s\n",
			j.ID, j.DomainID, j.Status, j.PagesCrawled,
			fmtTime(j.StartedAt), fmtTime(j.EndedAt),
		)
	}
	w.Flush()
}

func printJob(j Job) {
	if jsonOutput {
		printJSON(j)
		return
	}
	fmt.Printf("ID:           %d\n", j.ID)
	fmt.Printf("Domain ID:    %d\n", j.DomainID)
	fmt.Printf("Status:       %s\n", j.Status)
	fmt.Printf("Pages crawled:%d\n", j.PagesCrawled)
	fmt.Printf("Started:      %s\n", fmtTime(j.StartedAt))
	fmt.Printf("Ended:        %s\n", fmtTime(j.EndedAt))
	fmt.Printf("Created:      %s\n", fmtTime(&j.CreatedAt))
}

func printTokens(tokens []Token) {
	if jsonOutput {
		printJSON(tokens)
		return
	}
	w := newTable("ID", "REVOKED", "CREATED")
	for _, t := range tokens {
		fmt.Fprintf(w, "%d\t%s\t%s\n", t.ID, fmtBool(t.Revoked), fmtTime(&t.CreatedAt))
	}
	w.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
