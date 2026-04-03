package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"agentic-change-review-copilot/internal/review"
)

type ReviewHandler struct {
	service             *review.Service
	gitHubWebhookSecret string
}

type ReviewHandlerOption func(*ReviewHandler)

func WithReviewGitHubWebhookSecret(secret string) ReviewHandlerOption {
	return func(h *ReviewHandler) {
		h.gitHubWebhookSecret = strings.TrimSpace(secret)
	}
}

func NewReviewHandler(service *review.Service, options ...ReviewHandlerOption) http.Handler {
	handler := &ReviewHandler{service: service}
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	return handler
}

func (h *ReviewHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if r.URL.Path == "/api/v1/reviews" && r.Method == http.MethodPost {
		h.createReview(w, r)
		return
	}

	if r.URL.Path == "/api/v1/review-metrics" && r.Method == http.MethodGet {
		h.getMetrics(w, r)
		return
	}

	if r.URL.Path == "/api/v1/webhooks/github/pull-request" && r.Method == http.MethodPost {
		h.githubPullRequestWebhook(w, r)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/api/v1/reviews/") {
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/reviews/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	reviewID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		h.getReview(w, reviewID)
		return
	}

	if len(parts) != 2 {
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	switch {
	case parts[1] == "timeline" && r.Method == http.MethodGet:
		h.getTimeline(w, reviewID)
	case parts[1] == "decision" && r.Method == http.MethodPost:
		h.submitHumanDecision(w, r, reviewID)
	case parts[1] == "retry" && r.Method == http.MethodPost:
		h.retryReview(w, r, reviewID)
	case parts[1] == "report" && r.Method == http.MethodGet:
		h.exportReview(w, r, reviewID)
	case parts[1] == "evaluation" && r.Method == http.MethodPost:
		h.updateEvaluation(w, r, reviewID)
	default:
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *ReviewHandler) createReview(w http.ResponseWriter, r *http.Request) {
	var req review.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.SourceType == "" || req.SourceID == "" || req.Environment == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "source_type, source_id, and environment are required")
		return
	}

	resp, err := h.service.CreateReview(req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

func (h *ReviewHandler) getReview(w http.ResponseWriter, reviewID string) {
	resp, err := h.service.GetReview(reviewID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

func (h *ReviewHandler) getTimeline(w http.ResponseWriter, reviewID string) {
	resp, err := h.service.GetTimeline(reviewID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

func (h *ReviewHandler) submitHumanDecision(w http.ResponseWriter, r *http.Request, reviewID string) {
	var req review.HumanDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	resp, err := h.service.SubmitHumanDecision(reviewID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

func (h *ReviewHandler) retryReview(w http.ResponseWriter, r *http.Request, reviewID string) {
	var req review.RetryRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	resp, err := h.service.RetryReview(reviewID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

func (h *ReviewHandler) exportReview(w http.ResponseWriter, r *http.Request, reviewID string) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}

	body, contentType, err := h.service.ExportReview(reviewID, format)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *ReviewHandler) updateEvaluation(w http.ResponseWriter, r *http.Request, reviewID string) {
	var req review.EvaluationUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	resp, err := h.service.UpdateEvaluation(reviewID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

func (h *ReviewHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	service := r.URL.Query().Get("service")
	if from == "" || to == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "from and to are required")
		return
	}

	resp, err := h.service.GetEvaluationMetrics(from, to, service)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

func (h *ReviewHandler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, review.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, review.ErrConflict):
		h.writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

func (h *ReviewHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *ReviewHandler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, review.ErrorResponse{
		Code:    code,
		Message: message,
	})
}
