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

func TestCreateReviewAndFetch(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"source_type": "pull_request",
		"source_id":   "PR-123",
		"repo":        "gateway-service",
		"service":     "api-gateway",
		"environment": "prod",
		"payload": map[string]any{
			"title":  "adjust auth routing",
			"author": "alice",
			"metadata": map[string]any{
				"file_list": []string{"configs/routes.yaml", "gateway/auth.go"},
			},
		},
	}
	createPayload, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
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
		t.Fatal("review_id should not be empty")
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
		t.Fatal("risk_level should be populated")
	}
	if len(getResp.Signals) == 0 {
		t.Fatal("expected generated signals")
	}
	if !getResp.Review.HumanReviewRequired {
		t.Fatal("expected prod gateway review to require human review")
	}
}

func TestSubmitHumanDecision(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"source_type": "sql_migration",
		"source_id":   "SQL-1",
		"service":     "billing",
		"environment": "prod",
		"payload": map[string]any{
			"title":  "add index",
			"author": "alice",
		},
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	var createResp review.CreateReviewResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	decisionBody := map[string]any{
		"reviewer": "bob",
		"decision": "override",
		"reason":   "scheduled release window",
	}
	decisionPayload, _ := json.Marshal(decisionBody)
	decisionReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+createResp.ReviewID+"/human-decision", bytes.NewReader(decisionPayload))
	decisionReq.Header.Set("Content-Type", "application/json")
	decisionRec := httptest.NewRecorder()
	handler.ServeHTTP(decisionRec, decisionReq)

	if decisionRec.Code != http.StatusOK {
		t.Fatalf("decision status = %d, want %d, body=%s", decisionRec.Code, http.StatusOK, decisionRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+createResp.ReviewID+"/timeline", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	var timeline review.TimelineResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &timeline); err != nil {
		t.Fatalf("decode timeline response: %v", err)
	}
	if len(timeline.Events) == 0 {
		t.Fatal("expected timeline events")
	}
	if timeline.Events[len(timeline.Events)-1].State != review.StatusOverridden {
		t.Fatalf("last state = %s, want %s", timeline.Events[len(timeline.Events)-1].State, review.StatusOverridden)
	}
}

func TestUpdateEvaluationAffectsMetrics(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"source_type": "sql_migration",
		"source_id":   "SQL-2",
		"service":     "billing",
		"environment": "prod",
		"payload": map[string]any{
			"title":  "change billing schema",
			"author": "alice",
		},
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	var createResp review.CreateReviewResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	incidentFalse := false
	evaluationBody := map[string]any{
		"release_outcome": "success",
		"incident_flag":   incidentFalse,
		"outcome_metadata": map[string]any{
			"release_id": "rel-001",
		},
	}
	evaluationPayload, _ := json.Marshal(evaluationBody)
	evalReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+createResp.ReviewID+"/evaluation", bytes.NewReader(evaluationPayload))
	evalReq.Header.Set("Content-Type", "application/json")
	evalRec := httptest.NewRecorder()
	handler.ServeHTTP(evalRec, evalReq)

	if evalRec.Code != http.StatusOK {
		t.Fatalf("evaluation status = %d, want %d, body=%s", evalRec.Code, http.StatusOK, evalRec.Body.String())
	}

	today := time.Now().UTC().Format("2006-01-02")
	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/metrics?from="+today+"&to="+today+"&service=billing", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d, body=%s", metricsRec.Code, http.StatusOK, metricsRec.Body.String())
	}

	var metricsResp review.EvaluationMetricsResponse
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &metricsResp); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if metricsResp.Metrics.ReviewCount != 1 {
		t.Fatalf("review_count = %d, want 1", metricsResp.Metrics.ReviewCount)
	}
	if metricsResp.Metrics.FalsePositiveRate != 1 {
		t.Fatalf("false_positive_rate = %v, want 1", metricsResp.Metrics.FalsePositiveRate)
	}
}

func TestCreateReviewIsIdempotentByDedupeKey(t *testing.T) {
	service := review.NewService(review.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"source_type": "pull_request",
		"source_id":   "PR-456",
		"repo":        "gateway-service",
		"service":     "api-gateway",
		"environment": "prod",
		"dedupe_key":  "github:gateway-service:pr-456:def456:prod",
		"payload": map[string]any{
			"title":       "adjust auth routing",
			"author":      "alice",
			"base_commit": "abc123",
			"head_commit": "def456",
		},
	}
	createPayload, _ := json.Marshal(createBody)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	firstReq.Header.Set("Content-Type", "application/json")
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)

	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("first create status = %d, want %d, body=%s", firstRec.Code, http.StatusAccepted, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewReader(createPayload))
	secondReq.Header.Set("Content-Type", "application/json")
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)

	if secondRec.Code != http.StatusAccepted {
		t.Fatalf("second create status = %d, want %d, body=%s", secondRec.Code, http.StatusAccepted, secondRec.Body.String())
	}

	var firstResp review.CreateReviewResponse
	if err := json.Unmarshal(firstRec.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("decode first create response: %v", err)
	}
	var secondResp review.CreateReviewResponse
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("decode second create response: %v", err)
	}

	if firstResp.ReviewID != secondResp.ReviewID {
		t.Fatalf("review_id mismatch: first=%s second=%s", firstResp.ReviewID, secondResp.ReviewID)
	}
	if firstResp.TaskID != secondResp.TaskID {
		t.Fatalf("task_id mismatch: first=%s second=%s", firstResp.TaskID, secondResp.TaskID)
	}

	today := time.Now().UTC().Format("2006-01-02")
	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/evaluations/metrics?from="+today+"&to="+today+"&service=api-gateway", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d, body=%s", metricsRec.Code, http.StatusOK, metricsRec.Body.String())
	}

	var metricsResp review.EvaluationMetricsResponse
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &metricsResp); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if metricsResp.Metrics.ReviewCount != 1 {
		t.Fatalf("review_count = %d, want 1", metricsResp.Metrics.ReviewCount)
	}
}
