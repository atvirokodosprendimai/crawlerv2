package httpinfra

import (
	"net/http"
	"strconv"

	appCrawler "github.com/atvirokodosprendimai/crawlerv2/application/crawler"
	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
)

type JobHandlers struct {
	triggerUC *appCrawler.TriggerJobUseCase
	jobs      domain.CrawlJobRepository
}

func NewJobHandlers(trigger *appCrawler.TriggerJobUseCase, jobs domain.CrawlJobRepository) *JobHandlers {
	return &JobHandlers{triggerUC: trigger, jobs: jobs}
}

func (h *JobHandlers) Trigger(w http.ResponseWriter, r *http.Request) {
	domainID, err := urlParamUint(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid domain id")
		return
	}
	var req TriggerJobRequest
	_ = decodeJSON(r, &req)

	job, err := h.triggerUC.Execute(r.Context(), appCrawler.TriggerJobInput{
		DomainID: domainID,
		ExtractConfig: domain.ExtractConfig{
			ExtractTitle:   req.ExtractConfig.ExtractTitle,
			ExtractMeta:    req.ExtractConfig.ExtractMeta,
			ExtractBody:    req.ExtractConfig.ExtractBody,
			DownloadBinary: req.ExtractConfig.DownloadBinary,
			FilesDir:       req.ExtractConfig.FilesDir,
			MaxFileSizeMB:  req.ExtractConfig.MaxFileSizeMB,
		},
	})
	if err != nil {
		switch err {
		case appCrawler.ErrJobAlreadyRunning:
			jsonError(w, http.StatusConflict, err.Error())
		case appCrawler.ErrDomainNotFound:
			jsonError(w, http.StatusNotFound, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	jsonCreated(w, jobToResponse(job))
}

func (h *JobHandlers) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var domainID uint
	if s := q.Get("domain_id"); s != "" {
		n, _ := strconv.ParseUint(s, 10, 64)
		domainID = uint(n)
	}
	status := domain.JobStatus(q.Get("status"))

	jobs, err := h.jobs.FindAll(r.Context(), domainID, status)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]JobResponse, len(jobs))
	for i := range jobs {
		resp[i] = jobToResponse(&jobs[i])
	}
	jsonOK(w, resp)
}

func (h *JobHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamUint(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	job, err := h.jobs.FindByID(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}
	jsonOK(w, jobToResponse(job))
}
