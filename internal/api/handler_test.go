package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
