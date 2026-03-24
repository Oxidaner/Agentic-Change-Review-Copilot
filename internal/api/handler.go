package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"agentic-change-review-copilot/internal/testflow"
)

type Handler struct {
	service             *testflow.Service
	gitHubWebhookSecret string
}

type HandlerOption func(*Handler)

func WithGitHubWebhookSecret(secret string) HandlerOption {
	return func(h *Handler) {
		h.gitHubWebhookSecret = strings.TrimSpace(secret)
	}
}

// NewHandler returns the top-level HTTP handler for the testflow API.
//
// Routing is intentionally implemented with a small manual dispatcher because the
// current MVP surface is limited and does not yet justify a larger routing
// framework dependency.
func NewHandler(service *testflow.Service, options ...HandlerOption) http.Handler {
	handler := &Handler{service: service}
	for _, option := range options {
		if option != nil {
			option(handler)
		}
	}
	return handler
}

// ServeHTTP performs coarse path dispatch and delegates request-specific work to
// smaller handler methods.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		h.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if r.URL.Path == "/api/v1/test-tasks" && r.Method == http.MethodPost {
		h.createTask(w, r)
		return
	}

	if r.URL.Path == "/api/v1/test-metrics" && r.Method == http.MethodGet {
		h.getMetrics(w, r)
		return
	}

	if r.URL.Path == "/api/v1/webhooks/github/pull-request" && r.Method == http.MethodPost {
		h.githubPullRequestWebhook(w, r)
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/api/v1/test-tasks/") {
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/test-tasks/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	taskID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodGet {
		h.getTask(w, taskID)
		return
	}

	if len(parts) != 2 {
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}

	switch {
	case parts[1] == "timeline" && r.Method == http.MethodGet:
		h.getTimeline(w, taskID)
	case parts[1] == "retry" && r.Method == http.MethodPost:
		h.retryTask(w, r, taskID)
	case parts[1] == "report" && r.Method == http.MethodGet:
		h.exportReport(w, r, taskID)
	default:
		h.writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

// createTask validates and accepts a new test task request.
func (h *Handler) createTask(w http.ResponseWriter, r *http.Request) {
	var req testflow.CreateTestTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if req.InputType == "" || req.SourceID == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "input_type and source_id are required")
		return
	}

	resp, err := h.service.CreateTask(req)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

// getTask returns the latest materialized state of a test task.
func (h *Handler) getTask(w http.ResponseWriter, taskID string) {
	resp, err := h.service.GetTask(taskID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// getTimeline returns the ordered state transition history for a task.
func (h *Handler) getTimeline(w http.ResponseWriter, taskID string) {
	resp, err := h.service.GetTimeline(taskID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// retryTask replays a previously failed task by reusing the same task_id.
func (h *Handler) retryTask(w http.ResponseWriter, r *http.Request, taskID string) {
	var req testflow.RetryTaskRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	resp, err := h.service.RetryTask(taskID, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, resp)
}

// exportReport renders a task result as markdown or JSON for downstream sharing.
func (h *Handler) exportReport(w http.ResponseWriter, r *http.Request, taskID string) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "markdown"
	}

	body, contentType, err := h.service.ExportReport(taskID, format)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

// getMetrics exposes aggregated metrics over a time window.
func (h *Handler) getMetrics(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	scenario := r.URL.Query().Get("scenario")
	if from == "" || to == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "from and to are required")
		return
	}

	resp, err := h.service.GetMetrics(from, to, scenario)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	h.writeJSON(w, http.StatusOK, resp)
}

// writeServiceError maps domain/store errors onto HTTP responses.
func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, testflow.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, testflow.ErrConflict):
		h.writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		h.writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// writeJSON standardizes JSON responses for successful and error flows.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError emits the common API error envelope.
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, testflow.ErrorResponse{Code: code, Message: message})
}
