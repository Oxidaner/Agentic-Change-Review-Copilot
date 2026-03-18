package review

import "time"

type ReviewStatus string

const (
	StatusCreated            ReviewStatus = "CREATED"
	StatusNormalized         ReviewStatus = "NORMALIZED"
	StatusUnderstood         ReviewStatus = "UNDERSTOOD"
	StatusCollectingContext  ReviewStatus = "COLLECTING_CONTEXT"
	StatusContextReady       ReviewStatus = "CONTEXT_READY"
	StatusSignalsExtracted   ReviewStatus = "SIGNALS_EXTRACTED"
	StatusScored             ReviewStatus = "SCORED"
	StatusRecommended        ReviewStatus = "RECOMMENDED"
	StatusWaitingHumanReview ReviewStatus = "WAITING_HUMAN_REVIEW"
	StatusApproved           ReviewStatus = "APPROVED"
	StatusRejected           ReviewStatus = "REJECTED"
	StatusOverridden         ReviewStatus = "OVERRIDDEN"
	StatusFailed             ReviewStatus = "FAILED"
	StatusCancelled          ReviewStatus = "CANCELLED"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskMedium   RiskLevel = "MEDIUM"
	RiskHigh     RiskLevel = "HIGH"
	RiskCritical RiskLevel = "CRITICAL"
)

type CreateReviewRequest struct {
	SourceType  string        `json:"source_type"`
	SourceID    string        `json:"source_id"`
	Repo        string        `json:"repo,omitempty"`
	Service     string        `json:"service,omitempty"`
	Environment string        `json:"environment"`
	TriggerMode string        `json:"trigger_mode,omitempty"`
	DedupeKey   string        `json:"dedupe_key,omitempty"`
	TriggeredBy string        `json:"triggered_by,omitempty"`
	Payload     ReviewPayload `json:"payload"`
}

type ReviewPayload struct {
	Title      string         `json:"title,omitempty"`
	Author     string         `json:"author,omitempty"`
	BaseCommit string         `json:"base_commit,omitempty"`
	HeadCommit string         `json:"head_commit,omitempty"`
	DiffURL    string         `json:"diff_url,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type CreateReviewResponse struct {
	ReviewID string       `json:"review_id"`
	TaskID   string       `json:"task_id"`
	Status   ReviewStatus `json:"status"`
	PollURL  string       `json:"poll_url"`
}

type Review struct {
	ReviewID            string       `json:"review_id"`
	ChangeID            string       `json:"change_id,omitempty"`
	DedupeKey           string       `json:"-"`
	SourceType          string       `json:"source_type"`
	Repo                string       `json:"repo,omitempty"`
	Service             string       `json:"service,omitempty"`
	Environment         string       `json:"environment,omitempty"`
	Status              ReviewStatus `json:"status"`
	RiskLevel           RiskLevel    `json:"risk_level,omitempty"`
	Score               int          `json:"score,omitempty"`
	Confidence          float64      `json:"confidence,omitempty"`
	Summary             string       `json:"summary,omitempty"`
	HumanReviewRequired bool         `json:"human_review_required"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

type RiskSignal struct {
	SignalID     string    `json:"signal_id,omitempty"`
	SignalName   string    `json:"signal_name"`
	Severity     RiskLevel `json:"severity"`
	ScoreDelta   int       `json:"score_delta,omitempty"`
	Explanation  string    `json:"explanation,omitempty"`
	EvidenceRefs []string  `json:"evidence_refs,omitempty"`
}

type Recommendation struct {
	RequiredReviewers []string        `json:"required_reviewers,omitempty"`
	ReleaseWindow     string          `json:"release_window,omitempty"`
	RolloutStrategy   RolloutStrategy `json:"rollout_strategy,omitempty"`
	ObservabilityPlan []string        `json:"observability_plan,omitempty"`
}

type RolloutStrategy struct {
	Steps           []string `json:"steps,omitempty"`
	IntervalMinutes int      `json:"interval_minutes,omitempty"`
}

type RollbackPlan struct {
	TriggerCondition       string   `json:"trigger_condition,omitempty"`
	Steps                  []string `json:"steps,omitempty"`
	VersionToRestore       string   `json:"version_to_restore,omitempty"`
	VerificationMetrics    []string `json:"verification_metrics,omitempty"`
	RollbackDeadlineSecond int      `json:"rollback_deadline_seconds,omitempty"`
}

type EvidenceItem struct {
	EvidenceID     string         `json:"evidence_id"`
	Type           string         `json:"type"`
	Source         string         `json:"source"`
	Title          string         `json:"title,omitempty"`
	ContentSnippet string         `json:"content_snippet,omitempty"`
	ReferenceURL   string         `json:"reference_url,omitempty"`
	Confidence     float64        `json:"confidence,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type GetReviewResponse struct {
	Review         Review         `json:"review"`
	Signals        []RiskSignal   `json:"signals,omitempty"`
	Recommendation Recommendation `json:"recommendation,omitempty"`
	RollbackPlan   RollbackPlan   `json:"rollback_plan,omitempty"`
	Evidence       []EvidenceItem `json:"evidence,omitempty"`
}

type TimelineEvent struct {
	State  ReviewStatus `json:"state"`
	At     time.Time    `json:"at"`
	Detail string       `json:"detail,omitempty"`
}

type TimelineResponse struct {
	ReviewID string          `json:"review_id"`
	Events   []TimelineEvent `json:"events"`
}

type HumanDecisionRequest struct {
	Reviewer string `json:"reviewer"`
	Decision string `json:"decision"`
	Reason   string `json:"reason,omitempty"`
	Notify   bool   `json:"notify,omitempty"`
}

type HumanDecisionResponse struct {
	ReviewID string       `json:"review_id"`
	Status   ReviewStatus `json:"status"`
	Recorded bool         `json:"recorded"`
}

type HumanDecision struct {
	Reviewer     string    `json:"reviewer"`
	Decision     string    `json:"decision"`
	Reason       string    `json:"reason,omitempty"`
	OverrideFlag bool      `json:"override_flag"`
	CreatedAt    time.Time `json:"created_at"`
}

type RetryRequest struct {
	Reason string `json:"reason,omitempty"`
}

type RetryResponse struct {
	ReviewID string       `json:"review_id"`
	TaskID   string       `json:"task_id"`
	Status   ReviewStatus `json:"status"`
}

type EvaluationUpdateRequest struct {
	ReleaseOutcome  string         `json:"release_outcome,omitempty"`
	IncidentFlag    *bool          `json:"incident_flag,omitempty"`
	OutcomeMetadata map[string]any `json:"outcome_metadata,omitempty"`
}

type EvaluationUpdateResponse struct {
	ReviewID string `json:"review_id"`
	Recorded bool   `json:"recorded"`
}

type EvaluationMetricsResponse struct {
	Window  MetricsWindow `json:"window"`
	Service string        `json:"service,omitempty"`
	Metrics Metrics       `json:"metrics"`
}

type MetricsWindow struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Metrics struct {
	ReviewCount       int     `json:"review_count"`
	HighRiskRecall    float64 `json:"high_risk_recall"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	OverrideRate      float64 `json:"override_rate"`
	P95LatencyMS      int     `json:"p95_latency_ms"`
}

type EvaluationRecord struct {
	FinalHumanDecision *string        `json:"final_human_decision,omitempty"`
	ReleaseOutcome     *string        `json:"release_outcome,omitempty"`
	IncidentFlag       *bool          `json:"incident_flag,omitempty"`
	OutcomeMetadata    map[string]any `json:"outcome_metadata,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type ErrorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

type Record struct {
	Review         Review
	Request        *CreateReviewRequest
	TaskID         string
	Signals        []RiskSignal
	Recommendation Recommendation
	RollbackPlan   RollbackPlan
	Evidence       []EvidenceItem
	Timeline       []TimelineEvent
	HumanDecisions []HumanDecision
	Evaluation     *EvaluationRecord
	LastError      string
}
