package testflow

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateTaskExecutesLiveHTTPRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-100",
		Repo:      "gateway-service",
		Service:   "api-gateway",
		Payload: ChangeInputPayload{
			Title: "validate health endpoint",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "health check",
						"method":        "GET",
						"path":          "/healthz",
						"expect_status": 200,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if len(getResp.TestCases) != 1 {
		t.Fatalf("test cases count = %d, want 1", len(getResp.TestCases))
	}
	if getResp.TestCases[0].ToolName != "api_test_runner_http" {
		t.Fatalf("tool_name = %s, want api_test_runner_http", getResp.TestCases[0].ToolName)
	}
	if len(getResp.ExecutionResults) != 1 {
		t.Fatalf("execution results count = %d, want 1", len(getResp.ExecutionResults))
	}
	if getResp.ExecutionResults[0].Status != ExecutionPassed {
		t.Fatalf("execution status = %s, want %s", getResp.ExecutionResults[0].Status, ExecutionPassed)
	}
	if _, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_request"); !ok {
		t.Fatalf("expected http_request artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	}
	if _, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_response"); !ok {
		t.Fatalf("expected http_response artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	}
	if _, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	}
	if getResp.Task.HumanReviewRequired {
		t.Fatal("human_review_required should be false when all live checks pass")
	}
}

func TestCreateTaskExecutesMultiStepHTTPFlowWithExtractionAndAssertions(t *testing.T) {
	var loginBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			body, _ := io.ReadAll(r.Body)
			loginBody = string(body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"abc123","status":"ok"}`))
		case "/profile":
			if got := r.Header.Get("Authorization"); got != "Bearer abc123" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"missing token"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","user":{"id":"u-1","role":"admin"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-150",
		Repo:      "gateway-service",
		Service:   "auth-gateway",
		Payload: ChangeInputPayload{
			Title: "validate auth flow",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"variables": map[string]any{
					"username": "alice",
				},
				"requests": []any{
					map[string]any{
						"name":          "login",
						"method":        "POST",
						"path":          "/login",
						"expect_status": 200,
						"headers": map[string]any{
							"Content-Type": "application/json",
						},
						"body": map[string]any{
							"user": "{{username}}",
						},
						"expect_json": map[string]any{
							"token": "abc123",
						},
						"extract": map[string]any{
							"auth_token": "token",
						},
					},
					map[string]any{
						"name":          "profile",
						"method":        "GET",
						"path":          "/profile",
						"expect_status": 200,
						"headers": map[string]any{
							"Authorization": "Bearer {{auth_token}}",
						},
						"expect_body_contains": []any{"admin"},
						"expect_json": map[string]any{
							"status":    "ok",
							"user.id":   "u-1",
							"user.role": "admin",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}
	if !strings.Contains(loginBody, `"user":"alice"`) {
		t.Fatalf("login body = %s, want templated username", loginBody)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if len(getResp.ExecutionResults) != 2 {
		t.Fatalf("execution results count = %d, want 2", len(getResp.ExecutionResults))
	}
	for i, result := range getResp.ExecutionResults {
		if result.Status != ExecutionPassed {
			t.Fatalf("execution result %d status = %s, want %s", i, result.Status, ExecutionPassed)
		}
	}
	if !containsTag(getResp.TestCases[1].Tags, "multi-step") {
		t.Fatalf("expected multi-step tag, got %v", getResp.TestCases[1].Tags)
	}
	if !containsTag(getResp.TestCases[1].Tags, "payload-assert") {
		t.Fatalf("expected payload-assert tag, got %v", getResp.TestCases[1].Tags)
	}
	if getResp.Workflow.State != StatusDone {
		t.Fatalf("workflow state = %s, want %s", getResp.Workflow.State, StatusDone)
	}
	if len(getResp.Workflow.Tools) != 1 {
		t.Fatalf("workflow tool count = %d, want 1", len(getResp.Workflow.Tools))
	}
	if getResp.Workflow.Tools[0].ToolName != "api_test_runner_http" {
		t.Fatalf("workflow tool name = %s, want api_test_runner_http", getResp.Workflow.Tools[0].ToolName)
	}
	if getResp.ExecutionPlan.Mode != "live_http" {
		t.Fatalf("execution plan mode = %s, want live_http", getResp.ExecutionPlan.Mode)
	}
	if len(getResp.ExecutionPlan.Steps) != 2 {
		t.Fatalf("execution plan step count = %d, want 2", len(getResp.ExecutionPlan.Steps))
	}
	if len(getResp.TraceEvents) == 0 {
		t.Fatal("expected trace events")
	}
	if traceEvent, ok := findTraceEvent(getResp.TraceEvents, "tool_result", getResp.TestCases[1].CaseID); !ok {
		t.Fatalf("expected tool_result trace event for %s, got %v", getResp.TestCases[1].CaseID, getResp.TraceEvents)
	} else if traceEvent.ToolName != "api_test_runner_http" {
		t.Fatalf("trace event tool name = %s, want api_test_runner_http", traceEvent.ToolName)
	}
	if getResp.ExecutionPlan.Steps[0].StepID != "step_"+getResp.TestCases[0].CaseID {
		t.Fatalf("step id = %s, want %s", getResp.ExecutionPlan.Steps[0].StepID, "step_"+getResp.TestCases[0].CaseID)
	}
	if len(getResp.ExecutionPlan.Steps[0].VariableRefs) != 1 || getResp.ExecutionPlan.Steps[0].VariableRefs[0] != "username" {
		t.Fatalf("step 0 variable refs = %v, want [username]", getResp.ExecutionPlan.Steps[0].VariableRefs)
	}
	if len(getResp.ExecutionPlan.Steps[1].VariableRefs) != 1 || getResp.ExecutionPlan.Steps[1].VariableRefs[0] != "auth_token" {
		t.Fatalf("step 1 variable refs = %v, want [auth_token]", getResp.ExecutionPlan.Steps[1].VariableRefs)
	}
	if extractArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_extract"); !ok {
		t.Fatalf("expected http_extract artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(extractArtifact.Snippet, "auth_token=***redacted***") {
		t.Fatalf("extract artifact snippet = %s, want redacted auth token", extractArtifact.Snippet)
	}
	if requestArtifact, ok := findArtifact(getResp.ExecutionResults[1].Artifacts, "http_request"); !ok {
		t.Fatalf("expected http_request artifact, got %v", getResp.ExecutionResults[1].Artifacts)
	} else if !strings.Contains(requestArtifact.Snippet, "Authorization=***redacted***") {
		t.Fatalf("request artifact snippet = %s, want redacted authorization", requestArtifact.Snippet)
	}
}

func TestCreateTaskExecutesLiveHTTPSessionCookieFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "s-123", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/profile":
			cookie, err := r.Cookie("session_id")
			if err != nil || cookie.Value != "s-123" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"missing session"}`))
				return
			}
			if got := r.Header.Get("X-Session-Mirror"); got != "s-123" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"missing mirrored session"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","user":{"role":"admin"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-160",
		Repo:      "gateway-service",
		Service:   "auth-gateway",
		Payload: ChangeInputPayload{
			Title: "validate session cookie flow",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "login",
						"method":        "POST",
						"path":          "/login",
						"expect_status": 200,
						"expect_cookies": map[string]any{
							"session_id": "s-123",
						},
						"extract_cookies": map[string]any{
							"session_cookie": "session_id",
						},
					},
					map[string]any{
						"name":          "profile",
						"method":        "GET",
						"path":          "/profile",
						"expect_status": 200,
						"headers": map[string]any{
							"X-Session-Mirror": "{{session_cookie}}",
						},
						"expect_json": map[string]any{
							"status":    "ok",
							"user.role": "admin",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !containsTag(getResp.TestCases[0].Tags, "cookie-session") {
		t.Fatalf("expected cookie-session tag, got %v", getResp.TestCases[0].Tags)
	}
	if !containsString(getResp.ExecutionPlan.Steps[0].ExpectedArtifacts, "http_cookie") {
		t.Fatalf("expected http_cookie planned artifact, got %v", getResp.ExecutionPlan.Steps[0].ExpectedArtifacts)
	}
	if len(getResp.ExecutionPlan.Steps[1].VariableRefs) != 1 || getResp.ExecutionPlan.Steps[1].VariableRefs[0] != "session_cookie" {
		t.Fatalf("step 1 variable refs = %v, want [session_cookie]", getResp.ExecutionPlan.Steps[1].VariableRefs)
	}
	if cookieArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_cookie"); !ok {
		t.Fatalf("expected http_cookie artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(cookieArtifact.Snippet, "response={session_id=***redacted***}") || !strings.Contains(cookieArtifact.Snippet, "extracted={session_cookie=***redacted***}") {
		t.Fatalf("cookie artifact snippet = %s, want response and extraction evidence", cookieArtifact.Snippet)
	}
	if cookieArtifact, ok := findArtifact(getResp.ExecutionResults[1].Artifacts, "http_cookie"); !ok {
		t.Fatalf("expected http_cookie artifact, got %v", getResp.ExecutionResults[1].Artifacts)
	} else if !strings.Contains(cookieArtifact.Snippet, "request={session_id=***redacted***}") {
		t.Fatalf("cookie artifact snippet = %s, want request cookie evidence", cookieArtifact.Snippet)
	}
}

func TestCreateTaskLiveHTTPCookieAssertionFailureRequiresHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/login" {
			http.NotFound(w, r)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "unexpected", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-162",
		Repo:      "gateway-service",
		Service:   "auth-gateway",
		Payload: ChangeInputPayload{
			Title: "validate issued session cookie",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "login",
						"method":        "POST",
						"path":          "/login",
						"expect_status": 200,
						"expect_cookies": map[string]any{
							"session_id": "s-123",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "expected response cookie session_id") {
		t.Fatalf("unexpected execution summary: %s", getResp.ExecutionResults[0].Summary)
	}
	if getResp.FailureAnalysis.FailureType != "contract_regression" {
		t.Fatalf("failure_type = %s, want contract_regression", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_cookie") {
		t.Fatalf("evidence_refs = %v, want http_cookie ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "cookies=session_id") {
		t.Fatalf("assertion artifact snippet = %s, want cookie assertion evidence", assertionArtifact.Snippet)
	}
	if cookieArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_cookie"); !ok {
		t.Fatalf("expected http_cookie artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(cookieArtifact.Snippet, "response={session_id=***redacted***}") {
		t.Fatalf("cookie artifact snippet = %s, want response cookie evidence", cookieArtifact.Snippet)
	}
}

func TestCreateTaskExecutesLiveHTTPQueryAndHeaderExtractionFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/search" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("X-Request-Source"); got != "probe" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"missing request source"}`))
			return
		}
		switch {
		case r.URL.Query().Get("page") == "2":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Next-Cursor", "cursor-2")
			_, _ = w.Write([]byte(`{"items":[{"id":"item-2"}]}`))
		case r.URL.Query().Get("cursor") == "cursor-2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","cursor":"cursor-2"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"unexpected query"}`))
		}
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-165",
		Repo:      "gateway-service",
		Service:   "search-gateway",
		Payload: ChangeInputPayload{
			Title: "validate search pagination flow",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"variables": map[string]any{
					"page_num": "2",
				},
				"requests": []any{
					map[string]any{
						"name":   "search page",
						"method": "GET",
						"path":   "/search",
						"query": map[string]any{
							"page": "{{page_num}}",
						},
						"headers": map[string]any{
							"X-Request-Source": "probe",
						},
						"expect_status":       200,
						"expect_json_present": []any{"items.0.id"},
						"extract_headers": map[string]any{
							"next_cursor": "X-Next-Cursor",
						},
					},
					map[string]any{
						"name":   "follow cursor",
						"method": "GET",
						"path":   "/search",
						"query": map[string]any{
							"cursor": "{{next_cursor}}",
						},
						"headers": map[string]any{
							"X-Request-Source": "probe",
						},
						"expect_status": 200,
						"expect_json": map[string]any{
							"status": "ok",
							"cursor": "cursor-2",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if len(getResp.ExecutionResults) != 2 {
		t.Fatalf("execution results count = %d, want 2", len(getResp.ExecutionResults))
	}
	if !containsTag(getResp.TestCases[0].Tags, "query-params") {
		t.Fatalf("expected query-params tag, got %v", getResp.TestCases[0].Tags)
	}
	if len(getResp.ExecutionPlan.Steps[0].VariableRefs) != 1 || getResp.ExecutionPlan.Steps[0].VariableRefs[0] != "page_num" {
		t.Fatalf("step 0 variable refs = %v, want [page_num]", getResp.ExecutionPlan.Steps[0].VariableRefs)
	}
	if len(getResp.ExecutionPlan.Steps[1].VariableRefs) != 1 || getResp.ExecutionPlan.Steps[1].VariableRefs[0] != "next_cursor" {
		t.Fatalf("step 1 variable refs = %v, want [next_cursor]", getResp.ExecutionPlan.Steps[1].VariableRefs)
	}
	if requestArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_request"); !ok {
		t.Fatalf("expected http_request artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(requestArtifact.Snippet, "/search?page=2") {
		t.Fatalf("request artifact snippet = %s, want query evidence", requestArtifact.Snippet)
	}
	if extractArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_extract"); !ok {
		t.Fatalf("expected http_extract artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(extractArtifact.Snippet, "next_cursor=cursor-2") {
		t.Fatalf("extract artifact snippet = %s, want header extraction evidence", extractArtifact.Snippet)
	}
	if !strings.Contains(getResp.ExecutionResults[1].Summary, "/search?cursor=cursor-2") {
		t.Fatalf("summary = %s, want extracted query evidence", getResp.ExecutionResults[1].Summary)
	}
}

func TestCreateTaskLiveHTTPNegativeAssertionsPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthy" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","meta":{"count":1}}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-180",
		Repo:      "gateway-service",
		Service:   "schema-gateway",
		Payload: ChangeInputPayload{
			Title: "validate negative assertions",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":                     "healthy negative assertions",
						"method":                   "GET",
						"path":                     "/healthy",
						"expect_status":            200,
						"expect_headers_absent":    []any{"X-Debug-Trace"},
						"expect_body_not_contains": []any{"stacktrace", "panic"},
						"expect_json_present":      []any{"meta.count"},
						"expect_json_absent":       []any{"error", "debug.trace"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !containsTag(getResp.TestCases[0].Tags, "payload-assert") {
		t.Fatalf("expected payload-assert tag, got %v", getResp.TestCases[0].Tags)
	}
	if !containsTag(getResp.TestCases[0].Tags, "schema-assert") {
		t.Fatalf("expected schema-assert tag, got %v", getResp.TestCases[0].Tags)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "headers_absent=X-Debug-Trace") || !strings.Contains(assertionArtifact.Snippet, "body_not_contains=stacktrace,panic") || !strings.Contains(assertionArtifact.Snippet, "json_absent=error,debug.trace") {
		t.Fatalf("assertion artifact snippet = %s, want negative assertion evidence", assertionArtifact.Snippet)
	}
}

func TestCreateTaskLiveHTTPNegativeJSONAssertionFailureRequiresHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthy" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","debug":{"trace":"abc"}}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-185",
		Repo:      "gateway-service",
		Service:   "schema-gateway",
		Payload: ChangeInputPayload{
			Title: "validate removed debug field",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":               "healthy negative json assertion",
						"method":             "GET",
						"path":               "/healthy",
						"expect_status":      200,
						"expect_json_absent": []any{"debug.trace"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "expected JSON path debug.trace to be absent") {
		t.Fatalf("unexpected execution summary: %s", getResp.ExecutionResults[0].Summary)
	}
	if getResp.FailureAnalysis.FailureType != "contract_regression" {
		t.Fatalf("failure_type = %s, want contract_regression", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_assertion") {
		t.Fatalf("evidence_refs = %v, want http_assertion ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "json_absent=debug.trace") {
		t.Fatalf("assertion artifact snippet = %s, want json absence evidence", assertionArtifact.Snippet)
	}
}
func TestCreateTaskLiveHTTPSchemaAndHeaderAssertionsPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ready" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Flow-State", "ready")
		_, _ = w.Write([]byte(`{"status":"ok","meta":{"count":2},"items":[{"id":"a1"}]}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-175",
		Repo:      "gateway-service",
		Service:   "schema-gateway",
		Payload: ChangeInputPayload{
			Title: "validate readiness schema",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "ready schema",
						"method":        "GET",
						"path":          "/ready",
						"expect_status": 200,
						"expect_headers": map[string]any{
							"X-Flow-State": "ready",
						},
						"expect_json_present": []any{"meta.count", "items.0.id"},
						"expect_json_types": map[string]any{
							"status":     "string",
							"meta":       "object",
							"meta.count": "integer",
							"items":      "array",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !containsTag(getResp.TestCases[0].Tags, "schema-assert") {
		t.Fatalf("expected schema-assert tag, got %v", getResp.TestCases[0].Tags)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "headers=X-Flow-State") || !strings.Contains(assertionArtifact.Snippet, "json_types=items,meta,meta.count,status") {
		t.Fatalf("assertion artifact snippet = %s, want schema/header evidence", assertionArtifact.Snippet)
	}
}

func TestCreateTaskLiveHTTPHeaderAssertionFailureRequiresHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ready" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Flow-State", "warming")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-190",
		Repo:      "gateway-service",
		Service:   "schema-gateway",
		Payload: ChangeInputPayload{
			Title: "validate readiness header",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "ready header",
						"method":        "GET",
						"path":          "/ready",
						"expect_status": 200,
						"expect_headers": map[string]any{
							"X-Flow-State": "ready",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "expected response header X-Flow-State") {
		t.Fatalf("unexpected execution summary: %s", getResp.ExecutionResults[0].Summary)
	}
	if getResp.FailureAnalysis.FailureType != "contract_regression" {
		t.Fatalf("failure_type = %s, want contract_regression", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_assertion") {
		t.Fatalf("evidence_refs = %v, want http_assertion ref", getResp.FailureAnalysis.EvidenceRefs)
	}
}

func TestCreateTaskLiveHTTPFailureRequiresHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/users" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-200",
		Repo:      "gateway-service",
		Service:   "api-gateway",
		Payload: ChangeInputPayload{
			Title: "validate users endpoint",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "users endpoint",
						"method":        "GET",
						"path":          "/api/users",
						"expect_status": 200,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if getResp.AssertionResult.Status != ExecutionFailed {
		t.Fatalf("assertion status = %s, want %s", getResp.AssertionResult.Status, ExecutionFailed)
	}
	if getResp.AssertionResult.FailedCount != 1 {
		t.Fatalf("failed_count = %d, want 1", getResp.AssertionResult.FailedCount)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "expected status 200") {
		t.Fatalf("unexpected execution summary: %s", getResp.ExecutionResults[0].Summary)
	}
	if !getResp.Task.HumanReviewRequired {
		t.Fatal("human_review_required should be true when live check fails")
	}
	if getResp.FailureAnalysis.FailureType != "service_runtime_failure" {
		t.Fatalf("failure_type = %s, want service_runtime_failure", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_response") {
		t.Fatalf("evidence_refs = %v, want http_response ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_trace") {
		t.Fatalf("evidence_refs = %v, want http_trace ref", getResp.FailureAnalysis.EvidenceRefs)
	}
}

func TestCreateTaskLiveHTTPPayloadAssertionFailureRequiresHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"degraded"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-250",
		Repo:      "gateway-service",
		Service:   "api-gateway",
		Payload: ChangeInputPayload{
			Title: "validate readiness payload",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "readiness",
						"method":        "GET",
						"path":          "/healthz",
						"expect_status": 200,
						"expect_json": map[string]any{
							"status": "ok",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if getResp.ExecutionResults[0].Status != ExecutionFailed {
		t.Fatalf("execution status = %s, want %s", getResp.ExecutionResults[0].Status, ExecutionFailed)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "expected JSON path status") {
		t.Fatalf("unexpected execution summary: %s", getResp.ExecutionResults[0].Summary)
	}
	if getResp.FailureAnalysis.FailureType != "contract_regression" {
		t.Fatalf("failure_type = %s, want contract_regression", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_assertion") {
		t.Fatalf("evidence_refs = %v, want http_assertion ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_response") {
		t.Fatalf("evidence_refs = %v, want http_response ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "json_paths=status") {
		t.Fatalf("assertion artifact snippet = %s, want json path evidence", assertionArtifact.Snippet)
	}
}

func TestCreateTaskRetriesTransientLiveHTTPRequest(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/flaky" {
			http.NotFound(w, r)
			return
		}
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"warming"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-275",
		Repo:      "gateway-service",
		Service:   "api-gateway",
		Payload: ChangeInputPayload{
			Title: "retry transient readiness failure",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "flaky readiness",
						"method":        "GET",
						"path":          "/flaky",
						"expect_status": 200,
						"max_attempts":  2,
						"expect_json": map[string]any{
							"status": "ok",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusDone {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusDone)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempt count = %d, want 2", attempts.Load())
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !containsTag(getResp.TestCases[0].Tags, "retry") {
		t.Fatalf("expected retry tag, got %v", getResp.TestCases[0].Tags)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "after 2 attempt(s)") {
		t.Fatalf("summary = %s, want retry evidence", getResp.ExecutionResults[0].Summary)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "attempts=2") || !strings.Contains(assertionArtifact.Snippet, "max_attempts=2") {
		t.Fatalf("assertion artifact snippet = %s, want retry evidence", assertionArtifact.Snippet)
	}
}

func TestCreateTaskLiveHTTPTimeoutBudgetRetriesAndFails(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slow" {
			http.NotFound(w, r)
			return
		}
		attempts.Add(1)
		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-290",
		Repo:      "gateway-service",
		Service:   "api-gateway",
		Payload: ChangeInputPayload{
			Title: "timeout budget enforcement",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "slow readiness",
						"method":        "GET",
						"path":          "/slow",
						"expect_status": 200,
						"timeout_ms":    5,
						"max_attempts":  2,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempt count = %d, want 2", attempts.Load())
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !containsTag(getResp.TestCases[0].Tags, "timeout-budget") {
		t.Fatalf("expected timeout-budget tag, got %v", getResp.TestCases[0].Tags)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "request failed after 2 attempt(s):") {
		t.Fatalf("summary = %s, want retry failure evidence", getResp.ExecutionResults[0].Summary)
	}
	if getResp.FailureAnalysis.FailureType != "environment_or_connectivity" {
		t.Fatalf("failure_type = %s, want environment_or_connectivity", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_request") {
		t.Fatalf("evidence_refs = %v, want http_request ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_response") {
		t.Fatalf("evidence_refs = %v, want http_response ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "timeout_ms=5") || !strings.Contains(assertionArtifact.Snippet, "attempts=2") {
		t.Fatalf("assertion artifact snippet = %s, want timeout and retry evidence", assertionArtifact.Snippet)
	}
}

func TestCreateTaskLiveHTTPLatencyBudgetFailureRequiresHumanReview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/slow-ok" {
			http.NotFound(w, r)
			return
		}
		time.Sleep(25 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-305",
		Repo:      "gateway-service",
		Service:   "api-gateway",
		Payload: ChangeInputPayload{
			Title: "latency budget enforcement",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":            "slow but successful readiness",
						"method":          "GET",
						"path":            "/slow-ok",
						"expect_status":   200,
						"max_duration_ms": 5,
						"expect_json": map[string]any{
							"status": "ok",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !containsTag(getResp.TestCases[0].Tags, "latency-budget") {
		t.Fatalf("expected latency-budget tag, got %v", getResp.TestCases[0].Tags)
	}
	if !strings.Contains(getResp.ExecutionResults[0].Summary, "exceeded max_duration_ms budget") {
		t.Fatalf("summary = %s, want latency budget evidence", getResp.ExecutionResults[0].Summary)
	}
	if getResp.FailureAnalysis.FailureType != "performance_regression" {
		t.Fatalf("failure_type = %s, want performance_regression", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_assertion") {
		t.Fatalf("evidence_refs = %v, want http_assertion ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_response") {
		t.Fatalf("evidence_refs = %v, want http_response ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if assertionArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_assertion"); !ok {
		t.Fatalf("expected http_assertion artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(assertionArtifact.Snippet, "max_duration_ms=5") || !strings.Contains(assertionArtifact.Snippet, "duration_ms=") {
		t.Fatalf("assertion artifact snippet = %s, want latency evidence", assertionArtifact.Snippet)
	}
	if responseArtifact, ok := findArtifact(getResp.ExecutionResults[0].Artifacts, "http_response"); !ok {
		t.Fatalf("expected http_response artifact, got %v", getResp.ExecutionResults[0].Artifacts)
	} else if !strings.Contains(responseArtifact.Snippet, "duration_ms=") {
		t.Fatalf("response artifact snippet = %s, want duration evidence", responseArtifact.Snippet)
	}
}
func TestCreateTaskLiveHTTPAuthFailureUsesArtifactGroundedAttribution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"missing token"}`))
	}))
	defer server.Close()

	service := NewService(NewMemoryStore())
	resp, err := service.CreateTask(CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-LIVE-310",
		Repo:      "gateway-service",
		Service:   "auth-gateway",
		Payload: ChangeInputPayload{
			Title: "validate auth profile access",
			Metadata: map[string]any{
				"api_base_url": server.URL,
				"requests": []any{
					map[string]any{
						"name":          "profile without token",
						"method":        "GET",
						"path":          "/profile",
						"expect_status": 200,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	if resp.Status != StatusHumanReviewRequired {
		t.Fatalf("create status = %s, want %s", resp.Status, StatusHumanReviewRequired)
	}

	getResp, err := service.GetTask(resp.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if getResp.FailureAnalysis.FailureType != "authentication_regression" {
		t.Fatalf("failure_type = %s, want authentication_regression", getResp.FailureAnalysis.FailureType)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_request") {
		t.Fatalf("evidence_refs = %v, want http_request ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_response") {
		t.Fatalf("evidence_refs = %v, want http_response ref", getResp.FailureAnalysis.EvidenceRefs)
	}
	if !containsString(getResp.FailureAnalysis.EvidenceRefs, "artifact:"+getResp.TestCases[0].CaseID+":http_assertion") {
		t.Fatalf("evidence_refs = %v, want http_assertion ref", getResp.FailureAnalysis.EvidenceRefs)
	}
}
func findTraceEvent(events []TraceEvent, kind, caseID string) (TraceEvent, bool) {
	for _, event := range events {
		if event.Kind == kind && event.CaseID == caseID {
			return event, true
		}
	}
	return TraceEvent{}, false
}

func findArtifact(artifacts []ExecutionArtifact, artifactType string) (ExecutionArtifact, bool) {
	for _, artifact := range artifacts {
		if artifact.ArtifactType == artifactType {
			return artifact, true
		}
	}
	return ExecutionArtifact{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}
