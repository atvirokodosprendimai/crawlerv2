package httpinfra

import (
	"net/http"

	appCrawler "github.com/atvirokodosprendimai/crawlerv2/application/crawler"
	domain "github.com/atvirokodosprendimai/crawlerv2/domain/crawler"
)

type DomainHandlers struct {
	addUC    *appCrawler.AddDomainUseCase
	updateUC *appCrawler.UpdateDomainUseCase
	domains  domain.DomainRepository

	// Called when a domain's cron changes so the scheduler can re-register
	OnDomainSaved func(d *domain.CrawlDomain)
}

func NewDomainHandlers(
	add *appCrawler.AddDomainUseCase,
	upd *appCrawler.UpdateDomainUseCase,
	domains domain.DomainRepository,
) *DomainHandlers {
	return &DomainHandlers{addUC: add, updateUC: upd, domains: domains}
}

func (h *DomainHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req AddDomainRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Host == "" {
		jsonError(w, http.StatusBadRequest, "host required")
		return
	}

	d, err := h.addUC.Execute(r.Context(), appCrawler.AddDomainInput{
		Host:             req.Host,
		ScopeConfig:      scopeDTOtoDomain(req.ScopeConfig),
		PolitenessConfig: politenessDTOtoDomain(req.PolitenessConfig),
		CronExpression:   req.CronExpression,
	})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.OnDomainSaved != nil {
		h.OnDomainSaved(d)
	}
	resp := domainToResponse(d)
	jsonCreated(w, resp)
}

func (h *DomainHandlers) List(w http.ResponseWriter, r *http.Request) {
	domains, err := h.domains.FindAll(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]DomainResponse, len(domains))
	for i := range domains {
		resp[i] = domainToResponse(&domains[i])
	}
	jsonOK(w, resp)
}

func (h *DomainHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamUint(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req AddDomainRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	d, err := h.updateUC.Execute(r.Context(), id, appCrawler.AddDomainInput{
		Host:             req.Host,
		ScopeConfig:      scopeDTOtoDomain(req.ScopeConfig),
		PolitenessConfig: politenessDTOtoDomain(req.PolitenessConfig),
		CronExpression:   req.CronExpression,
	})
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	if h.OnDomainSaved != nil {
		h.OnDomainSaved(d)
	}
	jsonOK(w, domainToResponse(d))
}

func (h *DomainHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamUint(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.domains.Delete(r.Context(), id); err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func scopeDTOtoDomain(dto ScopeConfigDTO) domain.ScopeConfig {
	return domain.ScopeConfig{
		IncludeSubdomains: dto.IncludeSubdomains,
		PathFilter:        dto.PathFilter,
		MaxDepth:          dto.MaxDepth,
		MaxPagesPerRun:    dto.MaxPagesPerRun,
	}
}

func politenessDTOtoDomain(dto PolitenessConfigDTO) domain.PolitenessConfig {
	return domain.PolitenessConfig{
		RespectRobotsTxt:        dto.RespectRobotsTxt,
		CrawlDelayMs:            dto.CrawlDelayMs,
		MaxConcurrencyPerWorker: dto.MaxConcurrencyPerWorker,
		MaxConcurrencyGlobal:    dto.MaxConcurrencyGlobal,
	}
}
