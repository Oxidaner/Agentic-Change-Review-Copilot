package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"agentic-change-review-copilot/internal/api"
	"agentic-change-review-copilot/internal/testflow"
)

func TestCreateTaskAndFetch(t *testing.T) {
	service := testflow.NewService(testflow.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"input_type": "pull_request",
		"source_id":  "PR-123",
		"repo":       "gateway-service",
		"service":    "api-gateway",
		"payload": map[string]any{
			"title":       "adjust auth routing",
			"author":      "alice",
			"description": "route auth traffic through gateway",
		},
	}
	createPayload, _ := json.Marshal(createBody)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/test-tasks", bytes.NewReader(createPayload))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d, body=%s", createRec.Code, http.StatusAccepted, createRec.Body.String())
	}

	var createResp testflow.CreateTestTaskResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResp.TaskID == "" {
		t.Fatal("task_id should not be empty")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/test-tasks/"+createResp.TaskID, nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d, body=%s", getRec.Code, http.StatusOK, getRec.Body.String())
	}

	var getResp testflow.GetTestTaskResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if getResp.Task.Scenario != "api_regression" {
		t.Fatalf("scenario = %s, want api_regression", getResp.Task.Scenario)
	}
	if len(getResp.TestPoints) == 0 {
		t.Fatal("expected generated test points")
	}
	if len(getResp.TestCases) == 0 {
		t.Fatal("expected generated test cases")
	}
	if len(getResp.ExecutionResults) == 0 {
		t.Fatal("expected execution results")
	}
}

func TestTaskTimelineAndReport(t *testing.T) {
	service := testflow.NewService(testflow.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"input_type": "pull_request",
		"source_id":  "PR-456",
		"repo":       "gateway-service",
		"service":    "auth-gateway",
		"payload": map[string]any{
			"title":       "adjust auth routing",
			"author":      "alice",
			"description": "auth token route moved to gateway",
		},
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/test-tasks", bytes.NewReader(createPayload))
	createRec := httptest.NewRecorder()
	handler.ServeHTTP(createRec, createReq)

	var createResp testflow.CreateTestTaskResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	timelineReq := httptest.NewRequest(http.MethodGet, "/api/v1/test-tasks/"+createResp.TaskID+"/timeline", nil)
	timelineRec := httptest.NewRecorder()
	handler.ServeHTTP(timelineRec, timelineReq)

	if timelineRec.Code != http.StatusOK {
		t.Fatalf("timeline status = %d, want %d, body=%s", timelineRec.Code, http.StatusOK, timelineRec.Body.String())
	}

	var timeline testflow.TimelineResponse
	if err := json.Unmarshal(timelineRec.Body.Bytes(), &timeline); err != nil {
		t.Fatalf("decode timeline response: %v", err)
	}
	if len(timeline.Events) == 0 {
		t.Fatal("expected timeline events")
	}
	if timeline.Events[len(timeline.Events)-1].State != testflow.StatusHumanReviewRequired {
		t.Fatalf("last state = %s, want %s", timeline.Events[len(timeline.Events)-1].State, testflow.StatusHumanReviewRequired)
	}

	reportReq := httptest.NewRequest(http.MethodGet, "/api/v1/test-tasks/"+createResp.TaskID+"/report?format=json", nil)
	reportRec := httptest.NewRecorder()
	handler.ServeHTTP(reportRec, reportReq)

	if reportRec.Code != http.StatusOK {
		t.Fatalf("report status = %d, want %d, body=%s", reportRec.Code, http.StatusOK, reportRec.Body.String())
	}
}

func TestMetricsAndIdempotentCreate(t *testing.T) {
	service := testflow.NewService(testflow.NewMemoryStore())
	handler := api.NewHandler(service)

	createBody := map[string]any{
		"input_type": "pull_request",
		"source_id":  "PR-789",
		"repo":       "billing-service",
		"service":    "billing-api",
		"dedupe_key": "github:billing:pr-789:def456",
		"payload": map[string]any{
			"title":       "add billing endpoint regression checks",
			"author":      "alice",
			"head_commit": "def456",
		},
	}
	createPayload, _ := json.Marshal(createBody)

	firstReq := httptest.NewRequest(http.MethodPost, "/api/v1/test-tasks", bytes.NewReader(createPayload))
	firstRec := httptest.NewRecorder()
	handler.ServeHTTP(firstRec, firstReq)
	if firstRec.Code != http.StatusAccepted {
		t.Fatalf("first create status = %d, want %d, body=%s", firstRec.Code, http.StatusAccepted, firstRec.Body.String())
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/api/v1/test-tasks", bytes.NewReader(createPayload))
	secondRec := httptest.NewRecorder()
	handler.ServeHTTP(secondRec, secondReq)
	if secondRec.Code != http.StatusAccepted {
		t.Fatalf("second create status = %d, want %d, body=%s", secondRec.Code, http.StatusAccepted, secondRec.Body.String())
	}

	var firstResp testflow.CreateTestTaskResponse
	if err := json.Unmarshal(firstRec.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("decode first create response: %v", err)
	}
	var secondResp testflow.CreateTestTaskResponse
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondResp); err != nil {
		t.Fatalf("decode second create response: %v", err)
	}
	if firstResp.TaskID != secondResp.TaskID {
		t.Fatalf("task_id mismatch: first=%s second=%s", firstResp.TaskID, secondResp.TaskID)
	}

	today := time.Now().UTC().Format("2006-01-02")
	metricsReq := httptest.NewRequest(http.MethodGet, "/api/v1/test-metrics?from="+today+"&to="+today+"&scenario=api_regression", nil)
	metricsRec := httptest.NewRecorder()
	handler.ServeHTTP(metricsRec, metricsReq)

	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want %d, body=%s", metricsRec.Code, http.StatusOK, metricsRec.Body.String())
	}

	var metricsResp testflow.MetricsResponse
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &metricsResp); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if metricsResp.Metrics.TaskCount != 1 {
		t.Fatalf("task_count = %d, want 1", metricsResp.Metrics.TaskCount)
	}
}
