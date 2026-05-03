package httpinfra

import (
	"net/http"
	"strconv"

	appCrawler "github.com/atvirokodosprendimai/crawlerv2/application/crawler"
)

type WorkerHandlers struct {
	pollUC   *appCrawler.PollTasksUseCase
	submitUC *appCrawler.SubmitResultsUseCase
}

func NewWorkerHandlers(poll *appCrawler.PollTasksUseCase, submit *appCrawler.SubmitResultsUseCase) *WorkerHandlers {
	return &WorkerHandlers{pollUC: poll, submitUC: submit}
}

func (h *WorkerHandlers) Poll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var domainID uint
	if s := q.Get("domain_id"); s != "" {
		n, _ := strconv.ParseUint(s, 10, 64)
		domainID = uint(n)
	}

	batch := 10
	if s := q.Get("batch"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			batch = n
		}
	}

	tasks, err := h.pollUC.Execute(r.Context(), appCrawler.PollTasksInput{
		DomainID:  domainID,
		BatchSize: batch,
	})
	if err != nil {
		switch err {
		case appCrawler.ErrJobNotFound, appCrawler.ErrDomainNotFound:
			jsonError(w, http.StatusNotFound, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	if len(tasks) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]TaskResponse, len(tasks))
	for i, t := range tasks {
		resp[i] = TaskResponse{
			TaskID:  t.TaskID,
			URL:     t.URL,
			JobID:   t.JobID,
			Depth:   t.Depth,
			StoreID: t.StoreID,
			Extract: ExtractConfigDTO{
				ExtractTitle:   t.ExtractConfig.ExtractTitle,
				ExtractMeta:    t.ExtractConfig.ExtractMeta,
				ExtractBody:    t.ExtractConfig.ExtractBody,
				DownloadBinary: t.ExtractConfig.DownloadBinary,
				FilesDir:       t.ExtractConfig.FilesDir,
				MaxFileSizeMB:  t.ExtractConfig.MaxFileSizeMB,
			},
			Politeness: PolitenessConfigDTO{
				RespectRobotsTxt:        t.Politeness.RespectRobotsTxt,
				CrawlDelayMs:            t.Politeness.CrawlDelayMs,
				MaxConcurrencyPerWorker: t.Politeness.MaxConcurrencyPerWorker,
				MaxConcurrencyGlobal:    t.Politeness.MaxConcurrencyGlobal,
			},
		}
	}
	jsonOK(w, resp)
}

func (h *WorkerHandlers) SubmitResults(w http.ResponseWriter, r *http.Request) {
	jobID, err := urlParamUint(r, "jobID")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid jobID")
		return
	}

	var req SubmitResultsRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	inputs := make([]appCrawler.SubmitResultInput, len(req.Results))
	for i, item := range req.Results {
		inputs[i] = appCrawler.SubmitResultInput{
			TaskID:       item.TaskID,
			URL:          item.URL,
			Depth:        item.Depth,
			StatusCode:   item.StatusCode,
			ResponseTime: item.ResponseTime,
			ContentType:  item.ContentType,
			Title:        item.Title,
			MetaDesc:     item.MetaDesc,
			Body:         item.Body,
			Links:        item.Links,
			FilePath:     item.FilePath,
			FileSize:     item.FileSize,
			FileHash:     item.FileHash,
			StoreID:      item.StoreID,
			Error:        item.Error,
		}
	}

	if err := h.submitUC.Execute(r.Context(), jobID, inputs); err != nil {
		switch err {
		case appCrawler.ErrJobNotFound, appCrawler.ErrDomainNotFound:
			jsonError(w, http.StatusNotFound, err.Error())
		default:
			jsonError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
