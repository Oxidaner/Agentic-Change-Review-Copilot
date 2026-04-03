package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"agentic-change-review-copilot/internal/review"
)

type reviewWebhookIngestResponse struct {
	Accepted bool                `json:"accepted"`
	Ignored  bool                `json:"ignored,omitempty"`
	Reason   string              `json:"reason,omitempty"`
	ReviewID string              `json:"review_id,omitempty"`
	TaskID   string              `json:"task_id,omitempty"`
	Status   review.ReviewStatus `json:"status,omitempty"`
	PollURL  string              `json:"poll_url,omitempty"`
}

func (h *ReviewHandler) githubPullRequestWebhook(w http.ResponseWriter, r *http.Request) {
	if eventType := strings.TrimSpace(r.Header.Get("X-GitHub-Event")); eventType != "" && eventType != "pull_request" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "X-GitHub-Event must be pull_request for this endpoint")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "unable to read request body")
		return
	}
	if err := verifyGitHubWebhookSignature(h.gitHubWebhookSecret, body, r.Header.Get("X-Hub-Signature-256")); err != nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
		return
	}

	var event review.GitHubPullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	req, ignored, reason, err := review.GitHubPullRequestReviewRequest(event)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if ignored {
		h.writeJSON(w, http.StatusAccepted, reviewWebhookIngestResponse{
			Accepted: false,
			Ignored:  true,
			Reason:   reason,
		})
		return
	}

	resp, err := h.service.CreateReview(req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, reviewWebhookIngestResponse{
		Accepted: true,
		ReviewID: resp.ReviewID,
		TaskID:   resp.TaskID,
		Status:   resp.Status,
		PollURL:  resp.PollURL,
	})
}
