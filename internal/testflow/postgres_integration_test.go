package testflow

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPostgresServiceCreateIdempotentLifecycle(t *testing.T) {
	db := openPostgresIntegrationDB(t)
	service := NewService(NewPostgresStore(db))

	req := CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-PG-101",
		Repo:      "billing-service",
		Service:   "billing-api",
		Payload: ChangeInputPayload{
			Title:      "add billing endpoint regression checks",
			Author:     "alice",
			HeadCommit: "def456",
		},
		DedupeKey: "github:billing-service:pr-pg-101:def456",
	}

	first, err := service.CreateTask(req)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	second, err := service.CreateTask(req)
	if err != nil {
		t.Fatalf("create task second attempt: %v", err)
	}
	if first.TaskID != second.TaskID {
		t.Fatalf("task id mismatch: first=%s second=%s", first.TaskID, second.TaskID)
	}

	got, err := service.GetTask(first.TaskID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if got.Task.Status != StatusDone {
		t.Fatalf("task status = %s, want %s", got.Task.Status, StatusDone)
	}
	if len(got.TestPoints) == 0 {
		t.Fatal("expected generated test points")
	}
	if len(got.TestCases) == 0 {
		t.Fatal("expected generated test cases")
	}
	if len(got.ExecutionResults) == 0 {
		t.Fatal("expected execution results")
	}

	timeline, err := service.GetTimeline(first.TaskID)
	if err != nil {
		t.Fatalf("get timeline: %v", err)
	}
	if len(timeline.Events) == 0 {
		t.Fatal("expected timeline events")
	}
	if timeline.Events[len(timeline.Events)-1].State != StatusDone {
		t.Fatalf("last timeline state = %s, want %s", timeline.Events[len(timeline.Events)-1].State, StatusDone)
	}
}

func TestPostgresServiceRetryAndMetrics(t *testing.T) {
	db := openPostgresIntegrationDB(t)
	service := NewService(NewPostgresStore(db))

	req := CreateTestTaskRequest{
		InputType: "pull_request",
		SourceID:  "PR-PG-202",
		Repo:      "gateway-service",
		Service:   "auth-gateway",
		Payload: ChangeInputPayload{
			Title:       "adjust auth routing",
			Author:      "alice",
			HeadCommit:  "abc789",
			Description: "auth token route moved to gateway",
		},
	}

	createResp, err := service.CreateTask(req)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	before, err := service.GetTimeline(createResp.TaskID)
	if err != nil {
		t.Fatalf("get timeline before retry: %v", err)
	}

	retryResp, err := service.RetryTask(createResp.TaskID, RetryTaskRequest{Reason: "re-run for triage"})
	if err != nil {
		t.Fatalf("retry task: %v", err)
	}
	if retryResp.TaskID != createResp.TaskID {
		t.Fatalf("retry task id = %s, want %s", retryResp.TaskID, createResp.TaskID)
	}
	if retryResp.Status != StatusHumanReviewRequired {
		t.Fatalf("retry status = %s, want %s", retryResp.Status, StatusHumanReviewRequired)
	}

	after, err := service.GetTimeline(createResp.TaskID)
	if err != nil {
		t.Fatalf("get timeline after retry: %v", err)
	}
	if len(after.Events) <= len(before.Events) {
		t.Fatalf("timeline did not grow after retry: before=%d after=%d", len(before.Events), len(after.Events))
	}
	if !hasTimelineDetail(after.Events, "retry accepted") {
		t.Fatal("expected retry accepted event in timeline")
	}

	today := time.Now().UTC().Format("2006-01-02")
	metrics, err := service.GetMetrics(today, today, "api_regression")
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	if metrics.Metrics.TaskCount != 1 {
		t.Fatalf("task_count = %d, want 1", metrics.Metrics.TaskCount)
	}
	if metrics.Metrics.ManualInterventionRate != 1 {
		t.Fatalf("manual_intervention_rate = %v, want 1", metrics.Metrics.ManualInterventionRate)
	}
}

func openPostgresIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := db.Ping(); err != nil {
		t.Skipf("postgres not available for integration tests: %v", err)
	}

	if err := RunMigrations(db, migrationsDirForIntegrationTest(t)); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := db.Exec("TRUNCATE TABLE test_tasks"); err != nil {
		t.Fatalf("truncate test_tasks before test: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("TRUNCATE TABLE test_tasks")
	})

	return db
}

func migrationsDirForIntegrationTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve caller path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "migrations")
}

func hasTimelineDetail(events []TimelineEvent, detail string) bool {
	for _, event := range events {
		if event.Detail == detail {
			return true
		}
	}
	return false
}
