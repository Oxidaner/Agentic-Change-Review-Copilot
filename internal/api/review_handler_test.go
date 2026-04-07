package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentic-change-review-copilot/internal/api"
	"agentic-change-review-copilot/internal/review"
)

type stubAnalyzer struct {
	analysis review.HybridAnalysis
	err      error
}

func (s stubAnalyzer) Analyze(review.AnalyzerInput) (review.HybridAnalysis, error) {
	if s.err != nil {
		return review.HybridAnalysis{}, s.err
	}
	return s.analysis, nil
}

func TestReviewCreateGetTimelineAndDecision(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewReviewHandler(service)

	createBody := map[string]any{
		"source_type": "pull_request",
		"source_id":   "PR-123",
		"repo":        "gateway-service",
		"service":     "auth-gateway",
		"environment": "prod",
		"payload": map[string]any{
			"title":       "adjust auth routing",
			"author":      "alice",
			"base_commit": "abc123",
			"head_commit": "def456",
			"metadata": map[string]any{
				"file_list": []any{"configs/routes.yaml"},
				"cmdb": map[string]any{
					"service_tier": "tier-1",
					"owner":        "payments-platform",
				},
				"metrics": map[string]any{
					"summary": "error_rate stable, latency_p95 near threshold",
				},
				"runbook": map[string]any{
					"title": "Gateway rollback playbook",
					"url":   "https://runbooks.example.com/gateway/rollback",
				},
			},
		},
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d, body=%s", createRec.Code, http.StatusAccepted, createRec.Body.String())
	}

	var createResp review.CreateReviewResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.ReviewID == "" {
		t.Fatal("expected review id")
	}
	if createResp.Status != review.StatusWaitingHumanReview {
		t.Fatalf("status = %s, want %s", createResp.Status, review.StatusWaitingHumanReview)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+createResp.ReviewID, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	var getResp review.GetReviewResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.Review.RiskLevel == "" {
		t.Fatal("expected risk level")
	}
	if getResp.Analysis.Mode == "" {
		t.Fatal("expected hybrid analysis")
	}
	if len(getResp.Signals) == 0 {
		t.Fatal("expected risk signals")
	}
	if len(getResp.Evidence) < 4 {
		t.Fatalf("evidence count = %d, want at least 4", len(getResp.Evidence))
	}

	timelineReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+createResp.ReviewID+"/timeline", nil)
	timelineRec := httptest.NewRecorder()
	handler.ServeHTTP(timelineRec, timelineReq)
	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d, want %d, body=%s", timelineRec.Code, http.StatusOK, timelineRec.Body.String())
	}

	var timeline review.TimelineResponse
	if err := json.Unmarshal(timelineRec.Body.Bytes(), &timeline); err != nil {
		t.Fatalf("decode timeline response: %v", err)
	}
	if len(timeline.Events) == 0 {
		t.Fatal("expected timeline events")
	}
	if timeline.Events[len(timeline.Events)-1].State != review.StatusWaitingHumanReview {
		t.Fatalf("last timeline state = %s, want %s", timeline.Events[len(timeline.Events)-1].State, review.StatusWaitingHumanReview)
	}

	decisionBody := map[string]any{
		"reviewer": "release-manager",
		"decision": "approve",
		"reason":   "signals understood and rollback ready",
	}
	decisionPayload, _ := json.Marshal(decisionBody)
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+createResp.ReviewID+"/decision", bytes.NewReader(decisionPayload))
	decisionRec := httptest.NewRecorder()
	handler.ServeHTTP(decisionRec, decisionReq)
	if decisionRec.Code != http.StatusAccepted {
		t.Fatalf("decision status = %d, want %d, body=%s", decisionRec.Code, http.StatusAccepted, decisionRec.Body.String())
	}

	var decisionResp review.HumanDecisionResponse
	if err := json.Unmarshal(decisionRec.Body.Bytes(), &decisionResp); err != nil {
		t.Fatalf("decode decision response: %v", err)
	}
	if decisionResp.Status != review.StatusApproved {
		t.Fatalf("decision status = %s, want %s", decisionResp.Status, review.StatusApproved)
	}
}

func TestReviewGetIncludesHybridAnalysisFieldsFromAnalyzerResult(t *testing.T) {
	service := review.NewService(review.NewMemoryStore(), review.WithAnalyzer(stubAnalyzer{
		analysis: review.HybridAnalysis{
			Mode:                "rules_plus_llm_skeleton",
			Analyzer:            "stub_llm",
			Summary:             "Model flagged rollout-sensitive auth change.",
			Confidence:          0.91,
			RequiresHumanReview: true,
			Rationale:           []string{"auth path touches production gateway"},
			SuggestedSignals: []review.SuggestedRiskSignal{
				{
					SignalName:  "llm_gateway_auth_regression",
					Severity:    review.RiskHigh,
					Explanation: "Model detected gateway auth regression risk.",
				},
			},
		},
	}))
	handler := api.NewReviewHandler(service)

	createBody := map[string]any{
		"source_type": "pull_request",
		"source_id":   "PR-777",
		"repo":        "gateway-service",
		"service":     "gateway-service",
		"environment": "prod",
		"payload": map[string]any{
			"title":       "adjust auth routing",
			"author":      "alice",
			"base_commit": "abc123",
			"head_commit": "def456",
		},
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d, body=%s", createRec.Code, http.StatusAccepted, createRec.Body.String())
	}

	var createResp review.CreateReviewResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+createResp.ReviewID, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	var getResp review.GetReviewResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.Analysis.Analyzer != "stub_llm" {
		t.Fatalf("analysis analyzer = %s, want stub_llm", getResp.Analysis.Analyzer)
	}
	if getResp.Analysis.Summary != "Model flagged rollout-sensitive auth change." {
		t.Fatalf("analysis summary = %q, want model-derived summary", getResp.Analysis.Summary)
	}
	if getResp.Analysis.Confidence != 0.91 {
		t.Fatalf("analysis confidence = %v, want 0.91", getResp.Analysis.Confidence)
	}
	if !getResp.Analysis.RequiresHumanReview {
		t.Fatal("analysis requires_human_review = false, want true")
	}
	if len(getResp.Analysis.Rationale) != 1 || getResp.Analysis.Rationale[0] != "auth path touches production gateway" {
		t.Fatalf("analysis rationale = %+v, want analyzer rationale", getResp.Analysis.Rationale)
	}
	if len(getResp.Analysis.SuggestedSignals) != 1 {
		t.Fatalf("analysis suggested_signals = %+v, want 1 signal", getResp.Analysis.SuggestedSignals)
	}
	if getResp.Analysis.SuggestedSignals[0].SignalName != "llm_gateway_auth_regression" {
		t.Fatalf("analysis suggested_signal_name = %s, want llm_gateway_auth_regression", getResp.Analysis.SuggestedSignals[0].SignalName)
	}
}

func TestReviewWebhookCreatesReview(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewReviewHandler(service)

	payload := map[string]any{
		"action": "synchronize",
		"number": 42,
		"repository": map[string]any{
			"name":      "gateway-service",
			"full_name": "octo/gateway-service",
			"html_url":  "https://github.com/octo/gateway-service",
		},
		"pull_request": map[string]any{
			"number":   42,
			"title":    "adjust auth routing",
			"body":     "route auth traffic through gateway",
			"html_url": "https://github.com/octo/gateway-service/pull/42",
			"diff_url": "https://github.com/octo/gateway-service/pull/42.diff",
			"state":    "open",
			"user": map[string]any{
				"login": "alice",
			},
			"head": map[string]any{
				"ref": "feature/auth-routing",
				"sha": "def456",
			},
			"base": map[string]any{
				"ref": "main",
				"sha": "abc123",
			},
		},
		"sender": map[string]any{
			"login": "alice",
		},
	}
	requestBody, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/github/pull-request", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("webhook status = %d, want %d, body=%s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var resp struct {
		Accepted bool                `json:"accepted"`
		ReviewID string              `json:"review_id"`
		Status   review.ReviewStatus `json:"status"`
		PollURL  string              `json:"poll_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode webhook response: %v", err)
	}
	if !resp.Accepted || resp.ReviewID == "" {
		t.Fatalf("webhook response = %+v, want accepted review", resp)
	}

	getReq := httptest.NewRequest(http.MethodGet, resp.PollURL, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	var getResp review.GetReviewResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.Review.ChangeID != "octo/gateway-service#42" {
		t.Fatalf("change_id = %s, want octo/gateway-service#42", getResp.Review.ChangeID)
	}
	if getResp.Review.Environment != "prod" {
		t.Fatalf("environment = %s, want prod", getResp.Review.Environment)
	}
}

func TestReviewMetricsAndEvaluation(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewReviewHandler(service)

	resp, err := service.CreateReview(review.CreateReviewRequest{
		SourceType:  "pull_request",
		SourceID:    "PR-888",
		Repo:        "billing-service",
		Service:     "billing-service",
		Environment: "prod",
		Payload: review.ReviewPayload{
			Title:      "adjust billing auth",
			Author:     "alice",
			BaseCommit: "abc123",
			HeadCommit: "def456",
		},
	})
	if err != nil {
		t.Fatalf("seed review: %v", err)
	}

	updateBody := map[string]any{
		"release_outcome": "rolled_back",
		"incident_flag":   true,
		"outcome_metadata": map[string]any{
			"latency_regression": true,
		},
	}
	updatePayload, _ := json.Marshal(updateBody)
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+resp.ReviewID+"/evaluation", bytes.NewReader(updatePayload))
	updateRec := httptest.NewRecorder()
	handler.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusAccepted {
		t.Fatalf("evaluation status = %d, want %d, body=%s", updateRec.Code, http.StatusAccepted, updateRec.Body.String())
	}

	today := time.Now().UTC().Format("2006-01-02")
	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/review-metrics?from="+today+"&to="+today+"&service=billing-service", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d, body=%s", metricsRec.Code, http.StatusOK, metricsRec.Body.String())
	}

	var metricsResp review.EvaluationMetricsResponse
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &metricsResp); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if metricsResp.Metrics.ReviewCount == 0 {
		t.Fatal("expected review count")
	}
}
