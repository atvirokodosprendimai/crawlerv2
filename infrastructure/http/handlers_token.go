package httpinfra

import (
	"net/http"

	appCrawler "github.com/atvirokodosprendimai/crawlerv2/application/crawler"
)

type TokenHandlers struct {
	uc *appCrawler.ManageTokenUseCase
}

func NewTokenHandlers(uc *appCrawler.ManageTokenUseCase) *TokenHandlers {
	return &TokenHandlers{uc: uc}
}

func (h *TokenHandlers) Create(w http.ResponseWriter, r *http.Request) {
	plaintext, tok, err := h.uc.Create(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonCreated(w, CreateTokenResponse{
		TokenResponse: TokenResponse{
			ID:        tok.ID,
			Revoked:   tok.Revoked,
			CreatedAt: tok.CreatedAt,
		},
		Token: plaintext,
	})
}

func (h *TokenHandlers) Revoke(w http.ResponseWriter, r *http.Request) {
	id, err := urlParamUint(r, "id")
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.uc.Revoke(r.Context(), id); err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TokenHandlers) List(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.uc.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]TokenResponse, len(tokens))
	for i, t := range tokens {
		resp[i] = TokenResponse{ID: t.ID, Revoked: t.Revoked, CreatedAt: t.CreatedAt}
	}
	jsonOK(w, resp)
}
