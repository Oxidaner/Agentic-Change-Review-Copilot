package testflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func RunMigrations(db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil && !isAlreadyExistsError(err) {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func (s *PostgresStore) Save(record Record) error {
	requestJSON, err := json.Marshal(record.Request)
	if err != nil {
		return err
	}
	testPointsJSON, err := json.Marshal(record.TestPoints)
	if err != nil {
		return err
	}
	testCasesJSON, err := json.Marshal(record.TestCases)
	if err != nil {
		return err
	}
	resultsJSON, err := json.Marshal(record.ExecutionResults)
	if err != nil {
		return err
	}
	assertionJSON, err := json.Marshal(record.AssertionResult)
	if err != nil {
		return err
	}
	failureJSON, err := json.Marshal(record.FailureAnalysis)
	if err != nil {
		return err
	}
	reportJSON, err := json.Marshal(record.Report)
	if err != nil {
		return err
	}
	timelineJSON, err := json.Marshal(record.Timeline)
	if err != nil {
		return err
	}

	query := `
INSERT INTO test_tasks (
	task_id,
	dedupe_key,
	scenario,
	input_type,
	source_id,
	repo,
	service,
	status,
	overall_status,
	summary,
	confidence,
	human_review_required,
	input_payload,
	test_points,
	test_cases,
	execution_results,
	assertion_result,
	failure_analysis,
	report,
	timeline,
	created_at,
	updated_at
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb,$14::jsonb,$15::jsonb,$16::jsonb,$17::jsonb,$18::jsonb,$19::jsonb,$20::jsonb,$21,$22
)
ON CONFLICT (task_id) DO UPDATE SET
	dedupe_key = EXCLUDED.dedupe_key,
	scenario = EXCLUDED.scenario,
	input_type = EXCLUDED.input_type,
	source_id = EXCLUDED.source_id,
	repo = EXCLUDED.repo,
	service = EXCLUDED.service,
	status = EXCLUDED.status,
	overall_status = EXCLUDED.overall_status,
	summary = EXCLUDED.summary,
	confidence = EXCLUDED.confidence,
	human_review_required = EXCLUDED.human_review_required,
	input_payload = EXCLUDED.input_payload,
	test_points = EXCLUDED.test_points,
	test_cases = EXCLUDED.test_cases,
	execution_results = EXCLUDED.execution_results,
	assertion_result = EXCLUDED.assertion_result,
	failure_analysis = EXCLUDED.failure_analysis,
	report = EXCLUDED.report,
	timeline = EXCLUDED.timeline,
	updated_at = EXCLUDED.updated_at
`

	_, err = s.db.ExecContext(
		context.Background(),
		query,
		record.Task.TaskID,
		nullIfEmpty(record.Task.DedupeKey),
		record.Task.Scenario,
		record.Task.InputType,
		record.Task.ChangeID,
		nullIfEmpty(record.Task.Repo),
		nullIfEmpty(record.Task.Service),
		record.Task.Status,
		record.Task.OverallStatus,
		nullIfEmpty(record.Task.Summary),
		record.Task.Confidence,
		record.Task.HumanReviewRequired,
		jsonOrObject(requestJSON),
		jsonOrArray(testPointsJSON),
		jsonOrArray(testCasesJSON),
		jsonOrArray(resultsJSON),
		jsonOrObject(assertionJSON),
		jsonOrObject(failureJSON),
		jsonOrObject(reportJSON),
		jsonOrArray(timelineJSON),
		record.Task.CreatedAt,
		record.Task.UpdatedAt,
	)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

func (s *PostgresStore) Get(taskID string) (Record, error) {
	row := s.db.QueryRowContext(context.Background(), `
SELECT
	task_id,
	dedupe_key,
	scenario,
	input_type,
	source_id,
	repo,
	service,
	status,
	overall_status,
	summary,
	confidence,
	human_review_required,
	input_payload,
	test_points,
	test_cases,
	execution_results,
	assertion_result,
	failure_analysis,
	report,
	timeline,
	created_at,
	updated_at
FROM test_tasks
WHERE task_id = $1
`, taskID)
	return scanRecord(row)
}

func (s *PostgresStore) List() []Record {
	rows, err := s.db.QueryContext(context.Background(), `SELECT task_id FROM test_tasks ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var taskID string
		if err := rows.Scan(&taskID); err != nil {
			continue
		}
		record, err := s.Get(taskID)
		if err == nil {
			records = append(records, record)
		}
	}
	return records
}

func (s *PostgresStore) FindByDedupeKey(dedupeKey string) (Record, error) {
	row := s.db.QueryRowContext(context.Background(), `
SELECT
	task_id,
	dedupe_key,
	scenario,
	input_type,
	source_id,
	repo,
	service,
	status,
	overall_status,
	summary,
	confidence,
	human_review_required,
	input_payload,
	test_points,
	test_cases,
	execution_results,
	assertion_result,
	failure_analysis,
	report,
	timeline,
	created_at,
	updated_at
FROM test_tasks
WHERE dedupe_key = $1
`, dedupeKey)
	return scanRecord(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRecord(scanner rowScanner) (Record, error) {
	var (
		record         Record
		dedupeKey      sql.NullString
		repo           sql.NullString
		service        sql.NullString
		summary        sql.NullString
		requestJSON    []byte
		testPointsJSON []byte
		testCasesJSON  []byte
		resultsJSON    []byte
		assertionJSON  []byte
		failureJSON    []byte
		reportJSON     []byte
		timelineJSON   []byte
	)

	err := scanner.Scan(
		&record.Task.TaskID,
		&dedupeKey,
		&record.Task.Scenario,
		&record.Task.InputType,
		&record.Task.ChangeID,
		&repo,
		&service,
		&record.Task.Status,
		&record.Task.OverallStatus,
		&summary,
		&record.Task.Confidence,
		&record.Task.HumanReviewRequired,
		&requestJSON,
		&testPointsJSON,
		&testCasesJSON,
		&resultsJSON,
		&assertionJSON,
		&failureJSON,
		&reportJSON,
		&timelineJSON,
		&record.Task.CreatedAt,
		&record.Task.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Record{}, ErrNotFound
		}
		return Record{}, err
	}

	record.Task.DedupeKey = dedupeKey.String
	record.Task.Repo = repo.String
	record.Task.Service = service.String
	record.Task.Summary = summary.String
	record.Request = jsonOrObjectPtr[CreateTestTaskRequest](requestJSON)
	record.TestPoints = jsonOrArrayValue[TestPoint](testPointsJSON)
	record.TestCases = jsonOrArrayValue[TestCase](testCasesJSON)
	record.ExecutionResults = jsonOrArrayValue[ExecutionResult](resultsJSON)
	if value := jsonOrObjectPtr[AssertionResult](assertionJSON); value != nil {
		record.AssertionResult = *value
	}
	if value := jsonOrObjectPtr[FailureAnalysis](failureJSON); value != nil {
		record.FailureAnalysis = *value
	}
	if value := jsonOrObjectPtr[TestReport](reportJSON); value != nil {
		record.Report = *value
	}
	record.Timeline = jsonOrArrayValue[TimelineEvent](timelineJSON)
	return record, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func jsonOrObject(value []byte) []byte {
	if len(value) == 0 || string(value) == "null" {
		return []byte("{}")
	}
	return value
}

func jsonOrArray(value []byte) []byte {
	if len(value) == 0 || string(value) == "null" {
		return []byte("[]")
	}
	return value
}

func jsonOrObjectPtr[T any](payload []byte) *T {
	if len(payload) == 0 || string(payload) == "null" || string(payload) == "{}" {
		return nil
	}
	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil
	}
	return &value
}

func jsonOrArrayValue[T any](payload []byte) []T {
	if len(payload) == 0 || string(payload) == "null" {
		return nil
	}
	var value []T
	if err := json.Unmarshal(payload, &value); err != nil {
		return nil
	}
	return value
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505"
}
