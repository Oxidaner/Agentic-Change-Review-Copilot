package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunCreateReadsFromStdin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/test-tasks" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if !strings.Contains(string(body), `"source_id":"PR-CLI-100"`) {
			t.Fatalf("request body = %s, want source_id", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task_id":"tsk_100","status":"DONE","poll_url":"/api/v1/test-tasks/tsk_100","scenario":"api_regression"}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cliApp{
		client: server.Client(),
		stdin:  strings.NewReader(`{"input_type":"pull_request","source_id":"PR-CLI-100","payload":{"title":"cli create"}}`),
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := app.run([]string{"create", "-base-url", server.URL})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"task_id": "tsk_100"`) {
		t.Fatalf("stdout = %s, want formatted task_id", stdout.String())
	}
}

func TestRunCreateWaitFetchesTaskDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/test-tasks":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task_id":"tsk_wait","status":"DONE","poll_url":"/api/v1/test-tasks/tsk_wait","scenario":"api_regression"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/test-tasks/tsk_wait":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"task":{"task_id":"tsk_wait","scenario":"api_regression","status":"DONE","overall_status":"PASSED","created_at":"2026-03-24T00:00:00Z","updated_at":"2026-03-24T00:00:00Z","input_type":"pull_request"},"workflow":{"workflow_id":"wf_tsk_wait","name":"api_regression_workflow","state":"DONE"},"execution_plan":{"plan_id":"plan_tsk_wait","mode":"heuristic"},"trace_events":[],"assertion_result":{"status":"PASSED","passed_count":1,"failed_count":0,"false_positive":false},"failure_analysis":{"failure_type":"none"},"report":{"overall_status":"PASSED","total_cases":1,"passed_cases":1,"failed_cases":0}}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cliApp{
		client: server.Client(),
		stdin:  strings.NewReader(`{"input_type":"pull_request","source_id":"PR-CLI-150","payload":{"title":"cli wait"}}`),
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := app.run([]string{"create", "-base-url", server.URL, "-wait"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"workflow_id": "wf_tsk_wait"`) {
		t.Fatalf("stdout = %s, want workflow detail", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"plan_id": "plan_tsk_wait"`) {
		t.Fatalf("stdout = %s, want execution plan detail", stdout.String())
	}
}

func TestRunGetFetchesTaskDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/test-tasks/tsk_get" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"task":{"task_id":"tsk_get","scenario":"api_regression","status":"DONE","overall_status":"PASSED","created_at":"2026-03-24T00:00:00Z","updated_at":"2026-03-24T00:00:00Z","input_type":"pull_request"},"workflow":{"workflow_id":"wf_tsk_get","name":"api_regression_workflow","state":"DONE"},"execution_plan":{"plan_id":"plan_tsk_get","mode":"live_http"},"trace_events":[],"assertion_result":{"status":"PASSED","passed_count":1,"failed_count":0,"false_positive":false},"failure_analysis":{"failure_type":"none"},"report":{"overall_status":"PASSED","total_cases":1,"passed_cases":1,"failed_cases":0}}`))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cliApp{
		client: server.Client(),
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := app.run([]string{"get", "-base-url", server.URL, "-task-id", "tsk_get"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"task_id": "tsk_get"`) {
		t.Fatalf("stdout = %s, want task detail", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"mode": "live_http"`) {
		t.Fatalf("stdout = %s, want execution mode", stdout.String())
	}
}
func TestRunReportPrintsMarkdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/test-tasks/tsk_report/report" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		if got := r.URL.Query().Get("format"); got != "markdown" {
			t.Fatalf("report format = %s, want markdown", got)
		}
		w.Header().Set("Content-Type", "text/markdown")
		_, _ = w.Write([]byte("# Test Task tsk_report\n\n## Failure Analysis\n- Type: none\n"))
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := cliApp{
		client: server.Client(),
		stdin:  strings.NewReader(""),
		stdout: &stdout,
		stderr: &stderr,
	}

	exitCode := app.run([]string{"report", "-base-url", server.URL, "-task-id", "tsk_report"})
	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0, stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# Test Task tsk_report") {
		t.Fatalf("stdout = %s, want markdown report", stdout.String())
	}
}
