package testflow

import "time"

type TaskStatus string

const (
	StatusInit                TaskStatus = "INIT"
	StatusParseChange         TaskStatus = "PARSE_CHANGE"
	StatusExtractTestPoints   TaskStatus = "EXTRACT_TEST_POINTS"
	StatusGenerateTestCases   TaskStatus = "GENERATE_TEST_CASES"
	StatusPrepareEnv          TaskStatus = "PREPARE_ENV"
	StatusExecuteTools        TaskStatus = "EXECUTE_TOOLS"
	StatusSmartAssert         TaskStatus = "SMART_ASSERT"
	StatusRootCauseAnalyze    TaskStatus = "ROOT_CAUSE_ANALYZE"
	StatusGenerateReport      TaskStatus = "GENERATE_REPORT"
	StatusHumanReviewRequired TaskStatus = "HUMAN_REVIEW_REQUIRED"
	StatusDone                TaskStatus = "DONE"
	StatusFailed              TaskStatus = "FAILED"
)

type ExecutionStatus string

const (
	ExecutionPassed         ExecutionStatus = "PASSED"
	ExecutionFailed         ExecutionStatus = "FAILED"
	ExecutionPartial        ExecutionStatus = "PARTIAL"
	ExecutionNeedsAttention ExecutionStatus = "NEEDS_ATTENTION"
)

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

type ChangeInputPayload struct {
	Title       string         `json:"title,omitempty"`
	Author      string         `json:"author,omitempty"`
	BaseCommit  string         `json:"base_commit,omitempty"`
	HeadCommit  string         `json:"head_commit,omitempty"`
	Description string         `json:"description,omitempty"`
	DiffURL     string         `json:"diff_url,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type CreateTestTaskResponse struct {
	TaskID   string     `json:"task_id"`
	Status   TaskStatus `json:"status"`
	PollURL  string     `json:"poll_url"`
	Scenario string     `json:"scenario"`
}

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

type TestPoint struct {
	PointID      string   `json:"point_id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Rationale    string   `json:"rationale,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

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

type ExecutionArtifact struct {
	ArtifactType string `json:"artifact_type"`
	Location     string `json:"location,omitempty"`
	Snippet      string `json:"snippet,omitempty"`
}

type ExecutionResult struct {
	CaseID     string              `json:"case_id"`
	ToolName   string              `json:"tool_name"`
	Status     ExecutionStatus     `json:"status"`
	DurationMS int                 `json:"duration_ms"`
	Summary    string              `json:"summary,omitempty"`
	Artifacts  []ExecutionArtifact `json:"artifacts,omitempty"`
}

type AssertionResult struct {
	Status        ExecutionStatus `json:"status"`
	PassedCount   int             `json:"passed_count"`
	FailedCount   int             `json:"failed_count"`
	Summary       string          `json:"summary,omitempty"`
	FalsePositive bool            `json:"false_positive"`
}

type FailureAnalysis struct {
	FailureType       string   `json:"failure_type,omitempty"`
	ProbableRootCause string   `json:"probable_root_cause,omitempty"`
	EvidenceRefs      []string `json:"evidence_refs,omitempty"`
	Confidence        float64  `json:"confidence,omitempty"`
	NextAction        string   `json:"next_action,omitempty"`
}

type TestReport struct {
	OverallStatus     ExecutionStatus `json:"overall_status"`
	TotalCases        int             `json:"total_cases"`
	PassedCases       int             `json:"passed_cases"`
	FailedCases       int             `json:"failed_cases"`
	Summary           string          `json:"summary,omitempty"`
	RecommendedAction string          `json:"recommended_action,omitempty"`
}

type GetTestTaskResponse struct {
	Task             TestTask          `json:"task"`
	TestPoints       []TestPoint       `json:"test_points,omitempty"`
	TestCases        []TestCase        `json:"test_cases,omitempty"`
	ExecutionResults []ExecutionResult `json:"execution_results,omitempty"`
	AssertionResult  AssertionResult   `json:"assertion_result"`
	FailureAnalysis  FailureAnalysis   `json:"failure_analysis"`
	Report           TestReport        `json:"report"`
}

type TimelineEvent struct {
	State  TaskStatus `json:"state"`
	At     time.Time  `json:"at"`
	Detail string     `json:"detail,omitempty"`
}

type TimelineResponse struct {
	TaskID string          `json:"task_id"`
	Events []TimelineEvent `json:"events"`
}

type RetryTaskRequest struct {
	Reason string `json:"reason,omitempty"`
}

type RetryTaskResponse struct {
	TaskID string     `json:"task_id"`
	Status TaskStatus `json:"status"`
}

type MetricsResponse struct {
	Window   MetricsWindow `json:"window"`
	Scenario string        `json:"scenario,omitempty"`
	Metrics  Metrics       `json:"metrics"`
}

type MetricsWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Metrics struct {
	TaskCount                  int     `json:"task_count"`
	ExecutableCaseRate         float64 `json:"executable_case_rate"`
	WorkflowSuccessRate        float64 `json:"workflow_success_rate"`
	AssertionFalsePositiveRate float64 `json:"assertion_false_positive_rate"`
	AverageDurationMS          int     `json:"average_duration_ms"`
	AverageTokenCost           float64 `json:"average_token_cost"`
	ManualInterventionRate     float64 `json:"manual_intervention_rate"`
}

type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

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
