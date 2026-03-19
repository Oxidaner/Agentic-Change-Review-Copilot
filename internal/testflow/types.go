package testflow

import "time"

// TaskStatus tracks the lifecycle of the controlled testflow workflow.
type TaskStatus string

const (
	// StatusInit means the task was accepted and assigned an ID.
	StatusInit TaskStatus = "INIT"
	// StatusParseChange means the incoming change payload was normalized.
	StatusParseChange TaskStatus = "PARSE_CHANGE"
	// StatusExtractTestPoints means candidate validation angles were generated.
	StatusExtractTestPoints TaskStatus = "EXTRACT_TEST_POINTS"
	// StatusGenerateTestCases means executable test cases were produced.
	StatusGenerateTestCases TaskStatus = "GENERATE_TEST_CASES"
	// StatusPrepareEnv means the workflow prepared the execution environment.
	StatusPrepareEnv TaskStatus = "PREPARE_ENV"
	// StatusExecuteTools means tool-based test execution is in progress or done.
	StatusExecuteTools TaskStatus = "EXECUTE_TOOLS"
	// StatusSmartAssert means execution results were summarized into assertions.
	StatusSmartAssert TaskStatus = "SMART_ASSERT"
	// StatusRootCauseAnalyze means the workflow generated failure analysis output.
	StatusRootCauseAnalyze TaskStatus = "ROOT_CAUSE_ANALYZE"
	// StatusGenerateReport means the final report was assembled.
	StatusGenerateReport TaskStatus = "GENERATE_REPORT"
	// StatusHumanReviewRequired means automation stopped for manual triage.
	StatusHumanReviewRequired TaskStatus = "HUMAN_REVIEW_REQUIRED"
	// StatusDone means the workflow completed without further manual intervention.
	StatusDone   TaskStatus = "DONE"
	StatusFailed TaskStatus = "FAILED"
)

// ExecutionStatus is the normalized outcome used for test execution, assertions,
// and overall report status.
type ExecutionStatus string

const (
	ExecutionPassed         ExecutionStatus = "PASSED"
	ExecutionFailed         ExecutionStatus = "FAILED"
	ExecutionPartial        ExecutionStatus = "PARTIAL"
	ExecutionNeedsAttention ExecutionStatus = "NEEDS_ATTENTION"
)

// CreateTestTaskRequest is the external API payload for starting a testflow task.
type CreateTestTaskRequest struct {
	InputType   string             `json:"input_type"`
	SourceID    string             `json:"source_id"`
	Repo        string             `json:"repo,omitempty"`
	Service     string             `json:"service,omitempty"`
	Scenario    string             `json:"scenario,omitempty"`
	DedupeKey   string             `json:"dedupe_key,omitempty"`
	TriggeredBy string             `json:"triggered_by,omitempty"`
	Payload     ChangeInputPayload `json:"payload"`
}

// ChangeInputPayload carries source-specific metadata that helps derive test
// points and test cases.
type ChangeInputPayload struct {
	Title       string         `json:"title,omitempty"`
	Author      string         `json:"author,omitempty"`
	BaseCommit  string         `json:"base_commit,omitempty"`
	HeadCommit  string         `json:"head_commit,omitempty"`
	Description string         `json:"description,omitempty"`
	DiffURL     string         `json:"diff_url,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// CreateTestTaskResponse is returned once the task is accepted by the API.
type CreateTestTaskResponse struct {
	TaskID   string     `json:"task_id"`
	Status   TaskStatus `json:"status"`
	PollURL  string     `json:"poll_url"`
	Scenario string     `json:"scenario"`
}

// TestTask is the top-level materialized summary exposed by the API.
type TestTask struct {
	TaskID              string          `json:"task_id"`
	ChangeID            string          `json:"change_id,omitempty"`
	DedupeKey           string          `json:"-"`
	InputType           string          `json:"input_type"`
	Scenario            string          `json:"scenario"`
	Repo                string          `json:"repo,omitempty"`
	Service             string          `json:"service,omitempty"`
	Status              TaskStatus      `json:"status"`
	OverallStatus       ExecutionStatus `json:"overall_status"`
	Summary             string          `json:"summary,omitempty"`
	Confidence          float64         `json:"confidence,omitempty"`
	HumanReviewRequired bool            `json:"human_review_required"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// TestPoint captures one distinct validation angle extracted from the change.
type TestPoint struct {
	PointID      string   `json:"point_id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Rationale    string   `json:"rationale,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// TestCase is the executable artifact generated from a test point.
type TestCase struct {
	CaseID         string         `json:"case_id"`
	Title          string         `json:"title"`
	ToolName       string         `json:"tool_name"`
	Preconditions  []string       `json:"preconditions,omitempty"`
	Steps          []string       `json:"steps,omitempty"`
	InputData      map[string]any `json:"input_data,omitempty"`
	ExpectedResult []string       `json:"expected_result,omitempty"`
	Tags           []string       `json:"tags,omitempty"`
}

// ExecutionArtifact references execution byproducts such as traces or logs.
type ExecutionArtifact struct {
	ArtifactType string `json:"artifact_type"`
	Location     string `json:"location,omitempty"`
	Snippet      string `json:"snippet,omitempty"`
}

// ExecutionResult records the outcome of running one generated test case.
type ExecutionResult struct {
	CaseID     string              `json:"case_id"`
	ToolName   string              `json:"tool_name"`
	Status     ExecutionStatus     `json:"status"`
	DurationMS int                 `json:"duration_ms"`
	Summary    string              `json:"summary,omitempty"`
	Artifacts  []ExecutionArtifact `json:"artifacts,omitempty"`
}

// AssertionResult is the workflow's higher-level judgment over raw execution output.
type AssertionResult struct {
	Status        ExecutionStatus `json:"status"`
	PassedCount   int             `json:"passed_count"`
	FailedCount   int             `json:"failed_count"`
	Summary       string          `json:"summary,omitempty"`
	FalsePositive bool            `json:"false_positive"`
}

// FailureAnalysis stores the best-effort explanation generated for a failing or
// ambiguous workflow run.
type FailureAnalysis struct {
	FailureType       string   `json:"failure_type,omitempty"`
	ProbableRootCause string   `json:"probable_root_cause,omitempty"`
	EvidenceRefs      []string `json:"evidence_refs,omitempty"`
	Confidence        float64  `json:"confidence,omitempty"`
	NextAction        string   `json:"next_action,omitempty"`
}

// TestReport is the final user-facing task summary.
type TestReport struct {
	OverallStatus     ExecutionStatus `json:"overall_status"`
	TotalCases        int             `json:"total_cases"`
	PassedCases       int             `json:"passed_cases"`
	FailedCases       int             `json:"failed_cases"`
	Summary           string          `json:"summary,omitempty"`
	RecommendedAction string          `json:"recommended_action,omitempty"`
}

// GetTestTaskResponse is the aggregate response returned by GET /test-tasks/{id}.
type GetTestTaskResponse struct {
	Task             TestTask          `json:"task"`
	TestPoints       []TestPoint       `json:"test_points,omitempty"`
	TestCases        []TestCase        `json:"test_cases,omitempty"`
	ExecutionResults []ExecutionResult `json:"execution_results,omitempty"`
	AssertionResult  AssertionResult   `json:"assertion_result"`
	FailureAnalysis  FailureAnalysis   `json:"failure_analysis"`
	Report           TestReport        `json:"report"`
}

// TimelineEvent records one state transition or notable workflow event.
type TimelineEvent struct {
	State  TaskStatus `json:"state"`
	At     time.Time  `json:"at"`
	Detail string     `json:"detail,omitempty"`
}

// TimelineResponse returns the ordered event history for a task.
type TimelineResponse struct {
	TaskID string          `json:"task_id"`
	Events []TimelineEvent `json:"events"`
}

// RetryTaskRequest captures optional operator context when replaying a task.
type RetryTaskRequest struct {
	Reason string `json:"reason,omitempty"`
}

// RetryTaskResponse returns the reused identifier and current status after retry.
type RetryTaskResponse struct {
	TaskID string     `json:"task_id"`
	Status TaskStatus `json:"status"`
}

// MetricsResponse is the aggregate metrics view exposed by the testflow API.
type MetricsResponse struct {
	Window   MetricsWindow `json:"window"`
	Scenario string        `json:"scenario,omitempty"`
	Metrics  Metrics       `json:"metrics"`
}

// MetricsWindow defines the query time range for metrics aggregation.
type MetricsWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Metrics contains lightweight workflow quality and operational indicators.
type Metrics struct {
	TaskCount                  int     `json:"task_count"`
	ExecutableCaseRate         float64 `json:"executable_case_rate"`
	WorkflowSuccessRate        float64 `json:"workflow_success_rate"`
	AssertionFalsePositiveRate float64 `json:"assertion_false_positive_rate"`
	AverageDurationMS          int     `json:"average_duration_ms"`
	AverageTokenCost           float64 `json:"average_token_cost"`
	ManualInterventionRate     float64 `json:"manual_intervention_rate"`
}

// ErrorResponse is the common JSON error envelope for the HTTP API.
type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// Record is the internal persistence aggregate used by Store implementations.
//
// It keeps the top-level task summary together with all generated workflow
// artifacts so the API can reconstruct a task in one read.
type Record struct {
	Task             TestTask
	Request          *CreateTestTaskRequest
	TestPoints       []TestPoint
	TestCases        []TestCase
	ExecutionResults []ExecutionResult
	AssertionResult  AssertionResult
	FailureAnalysis  FailureAnalysis
	Report           TestReport
	Timeline         []TimelineEvent
	LastError        string
}
