package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"agentic-change-review-copilot/internal/testflow"
)

type webhookIngestResponse struct {
	Accepted bool                `json:"accepted"`
	Ignored  bool                `json:"ignored,omitempty"`
	Reason   string              `json:"reason,omitempty"`
	TaskID   string              `json:"task_id,omitempty"`
	Status   testflow.TaskStatus `json:"status,omitempty"`
	PollURL  string              `json:"poll_url,omitempty"`
	Scenario string              `json:"scenario,omitempty"`
}

func (h *Handler) githubPullRequestWebhook(w http.ResponseWriter, r *http.Request) {
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

	var event testflow.GitHubPullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	req, ignored, reason, err := testflow.GitHubPullRequestTaskRequest(event)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if ignored {
		h.writeJSON(w, http.StatusAccepted, webhookIngestResponse{
			Accepted: false,
			Ignored:  true,
			Reason:   reason,
		})
		return
	}

	resp, err := h.service.CreateTask(req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	h.writeJSON(w, http.StatusAccepted, webhookIngestResponse{
		Accepted: true,
		TaskID:   resp.TaskID,
		Status:   resp.Status,
		PollURL:  resp.PollURL,
		Scenario: resp.Scenario,
	})
}

func verifyGitHubWebhookSignature(secret string, body []byte, signatureHeader string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil
	}

	signatureHeader = strings.TrimSpace(signatureHeader)
	if signatureHeader == "" {
		return errors.New("missing X-Hub-Signature-256 header")
	}

	const prefix = "sha256="
	if len(signatureHeader) <= len(prefix) || !strings.EqualFold(signatureHeader[:len(prefix)], prefix) {
		return errors.New("invalid X-Hub-Signature-256 format")
	}

	signature, err := hex.DecodeString(signatureHeader[len(prefix):])
	if err != nil {
		return errors.New("invalid X-Hub-Signature-256 format")
	}

	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return errors.New("invalid X-Hub-Signature-256 signature")
	}
	return nil
}
