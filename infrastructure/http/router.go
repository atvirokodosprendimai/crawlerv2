package httpinfra

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
)

func NewRouter(
	tokens domain.TokenRepository,
	domains *DomainHandlers,
	jobs *JobHandlers,
	tok *TokenHandlers,
	worker *WorkerHandlers,
) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	auth := BearerAuthMiddleware(tokens)

	r.Group(func(r chi.Router) {
		r.Use(auth)

		// Domain management
		r.Post("/api/domains", domains.Create)
		r.Get("/api/domains", domains.List)
		r.Put("/api/domains/{id}", domains.Update)
		r.Delete("/api/domains/{id}", domains.Delete)

		// Job management
		r.Post("/api/domains/{id}/jobs", jobs.Trigger)
		r.Get("/api/jobs", jobs.List)
		r.Get("/api/jobs/{id}", jobs.Get)

		// Token management
		r.Post("/api/tokens", tok.Create)
		r.Get("/api/tokens", tok.List)
		r.Delete("/api/tokens/{id}", tok.Revoke)

		// Worker endpoints
		r.Get("/worker/tasks/next", worker.Poll)
		r.Post("/worker/jobs/{jobID}/results", worker.SubmitResults)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return r
}
