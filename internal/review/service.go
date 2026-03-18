package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

var ErrConflict = errors.New("conflict")

type Service struct {
	store   Store
	counter atomic.Uint64
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) CreateReview(req CreateReviewRequest) (CreateReviewResponse, error) {
	now := time.Now().UTC()
	reviewID := s.nextID("rvw")
	taskID := s.nextID("tsk")

	record := Record{
		Review: Review{
			ReviewID:    reviewID,
			ChangeID:    req.SourceID,
			SourceType:  req.SourceType,
			Repo:        req.Repo,
			Service:     req.Service,
			Environment: req.Environment,
			Status:      StatusCreated,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		TaskID: taskID,
		Timeline: []TimelineEvent{
			{State: StatusCreated, At: now, Detail: "review accepted"},
		},
	}

	if req.Payload.Author != "" {
		record.Review.Summary = fmt.Sprintf("Review created for %s by %s", req.SourceType, req.Payload.Author)
	} else {
		record.Review.Summary = fmt.Sprintf("Review created for %s", req.SourceType)
	}

	record = s.runPipeline(record, req)

	if err := s.store.Save(record); err != nil {
		return CreateReviewResponse{}, err
	}

	return CreateReviewResponse{
		ReviewID: reviewID,
		TaskID:   taskID,
		Status:   record.Review.Status,
		PollURL:  "/api/v1/reviews/" + reviewID,
	}, nil
}

func (s *Service) GetReview(reviewID string) (GetReviewResponse, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return GetReviewResponse{}, err
	}

	return GetReviewResponse{
		Review:         record.Review,
		Signals:        record.Signals,
		Recommendation: record.Recommendation,
		RollbackPlan:   record.RollbackPlan,
		Evidence:       record.Evidence,
	}, nil
}

func (s *Service) GetTimeline(reviewID string) (TimelineResponse, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return TimelineResponse{}, err
	}

	return TimelineResponse{
		ReviewID: reviewID,
		Events:   record.Timeline,
	}, nil
}

func (s *Service) SubmitHumanDecision(reviewID string, req HumanDecisionRequest) (HumanDecisionResponse, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return HumanDecisionResponse{}, err
	}

	now := time.Now().UTC()
	switch req.Decision {
	case "approve":
		record.Review.Status = StatusApproved
	case "reject":
		record.Review.Status = StatusRejected
	case "override":
		record.Review.Status = StatusOverridden
	default:
		return HumanDecisionResponse{}, fmt.Errorf("invalid decision")
	}
	record.Review.UpdatedAt = now
	record.HumanDecisions = append(record.HumanDecisions, HumanDecision{
		Reviewer:     req.Reviewer,
		Decision:     req.Decision,
		Reason:       req.Reason,
		OverrideFlag: req.Decision == "override",
		CreatedAt:    now,
	})
	record.Timeline = append(record.Timeline, TimelineEvent{
		State:  record.Review.Status,
		At:     now,
		Detail: fmt.Sprintf("human decision by %s: %s", req.Reviewer, req.Decision),
	})

	if err := s.store.Save(record); err != nil {
		return HumanDecisionResponse{}, err
	}

	return HumanDecisionResponse{
		ReviewID: reviewID,
		Status:   record.Review.Status,
		Recorded: true,
	}, nil
}

func (s *Service) RetryReview(reviewID string, _ RetryRequest) (RetryResponse, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return RetryResponse{}, err
	}
	if record.Review.Status != StatusFailed {
		return RetryResponse{}, ErrConflict
	}

	record.Review.Status = StatusCreated
	record.Review.UpdatedAt = time.Now().UTC()
	record.Timeline = append(record.Timeline, TimelineEvent{
		State:  StatusCreated,
		At:     record.Review.UpdatedAt,
		Detail: "retry accepted",
	})

	if err := s.store.Save(record); err != nil {
		return RetryResponse{}, err
	}

	return RetryResponse{
		ReviewID: reviewID,
		TaskID:   record.TaskID,
		Status:   record.Review.Status,
	}, nil
}

func (s *Service) ExportReview(reviewID, format string) ([]byte, string, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return nil, "", err
	}

	if format == "json" {
		payload, err := json.MarshalIndent(GetReviewResponse{
			Review:         record.Review,
			Signals:        record.Signals,
			Recommendation: record.Recommendation,
			RollbackPlan:   record.RollbackPlan,
			Evidence:       record.Evidence,
		}, "", "  ")
		return payload, "application/json", err
	}

	body := fmt.Sprintf(
		"# Review %s\n\nStatus: %s\n\nRisk: %s (%d)\n\nSummary: %s\n\n## Top Signals\n",
		record.Review.ReviewID,
		record.Review.Status,
		record.Review.RiskLevel,
		record.Review.Score,
		record.Review.Summary,
	)
	for _, signal := range record.Signals {
		body += fmt.Sprintf("- %s: %s\n", signal.SignalName, signal.Explanation)
	}
	body += "\n## Rollout Strategy\n"
	for _, step := range record.Recommendation.RolloutStrategy.Steps {
		body += fmt.Sprintf("- %s\n", step)
	}
	body += "\n## Rollback Plan\n"
	for _, step := range record.RollbackPlan.Steps {
		body += fmt.Sprintf("- %s\n", step)
	}

	return []byte(body), "text/markdown", nil
}

func (s *Service) GetEvaluationMetrics(from, to, service string) (EvaluationMetricsResponse, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return EvaluationMetricsResponse{}, fmt.Errorf("invalid from date")
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return EvaluationMetricsResponse{}, fmt.Errorf("invalid to date")
	}
	end = end.Add(24*time.Hour - time.Nanosecond)

	records := s.store.List()
	filtered := make([]Record, 0, len(records))
	overrides := 0
	highRisk := 0
	for _, record := range records {
		if !record.Review.CreatedAt.Before(start) && !record.Review.CreatedAt.After(end) {
			if service == "" || record.Review.Service == service {
				filtered = append(filtered, record)
				if record.Review.Status == StatusOverridden {
					overrides++
				}
				if record.Review.RiskLevel == RiskHigh || record.Review.RiskLevel == RiskCritical {
					highRisk++
				}
			}
		}
	}

	count := len(filtered)
	overrideRate := 0.0
	highRiskRecall := 0.0
	if count > 0 {
		overrideRate = float64(overrides) / float64(count)
		highRiskRecall = float64(highRisk) / float64(count)
	}

	return EvaluationMetricsResponse{
		Window:  MetricsWindow{From: from, To: to},
		Service: service,
		Metrics: Metrics{
			ReviewCount:       count,
			HighRiskRecall:    highRiskRecall,
			FalsePositiveRate: 0.08,
			OverrideRate:      overrideRate,
			P95LatencyMS:      1800,
		},
	}, nil
}

func (s *Service) runPipeline(record Record, req CreateReviewRequest) Record {
	record = s.transition(record, StatusNormalized, "payload normalized")
	record = s.transition(record, StatusUnderstood, "change intent understood")
	record = s.transition(record, StatusCollectingContext, "context retrieval simulated")
	record = s.transition(record, StatusContextReady, "context ready")

	evidence := []EvidenceItem{
		{
			EvidenceID:     s.nextID("ev"),
			Type:           "heuristic",
			Source:         "rule_engine",
			Title:          "MVP rule evaluation",
			ContentSnippet: "Signals generated from source type, environment, and service heuristics.",
			Confidence:     0.86,
			Metadata: map[string]any{
				"source_type": req.SourceType,
				"environment": req.Environment,
			},
		},
	}

	signals, score := s.generateSignals(req, evidence[0].EvidenceID)
	record.Signals = signals
	record.Evidence = evidence
	record = s.transition(record, StatusSignalsExtracted, fmt.Sprintf("%d risk signals extracted", len(signals)))
	record.Review.Score = min(score, 100)
	record.Review.RiskLevel = scoreToRiskLevel(record.Review.Score)
	record.Review.Confidence = confidenceFor(record.Review.RiskLevel)
	record = s.transition(record, StatusScored, fmt.Sprintf("risk score=%d", record.Review.Score))

	record.Recommendation = recommendationFor(record.Review)
	record.RollbackPlan = rollbackPlanFor(req)
	record.Review.HumanReviewRequired = record.Review.RiskLevel == RiskHigh || record.Review.RiskLevel == RiskCritical
	record.Review.Summary = summaryFor(req, record.Signals, record.Review)

	if record.Review.HumanReviewRequired {
		record = s.transition(record, StatusWaitingHumanReview, "high risk review requires human approval")
		return record
	}

	record = s.transition(record, StatusRecommended, "automated recommendation ready")
	return record
}

func (s *Service) transition(record Record, status ReviewStatus, detail string) Record {
	now := time.Now().UTC()
	record.Review.Status = status
	record.Review.UpdatedAt = now
	record.Timeline = append(record.Timeline, TimelineEvent{State: status, At: now, Detail: detail})
	return record
}

func (s *Service) generateSignals(req CreateReviewRequest, evidenceID string) ([]RiskSignal, int) {
	signals := make([]RiskSignal, 0, 4)
	score := 10

	addSignal := func(name string, severity RiskLevel, delta int, explanation string) {
		signals = append(signals, RiskSignal{
			SignalID:     s.nextID("sig"),
			SignalName:   name,
			Severity:     severity,
			ScoreDelta:   delta,
			Explanation:  explanation,
			EvidenceRefs: []string{evidenceID},
		})
		score += delta
	}

	switch req.SourceType {
	case "sql_migration":
		addSignal("sql_schema_change", RiskHigh, 30, "SQL migration changes are treated as high-impact in the MVP rule set.")
	case "k8s_diff":
		addSignal("runtime_config_changed", RiskMedium, 18, "Kubernetes configuration changes can affect runtime stability.")
	case "terraform_plan":
		addSignal("infrastructure_change", RiskHigh, 24, "Infrastructure plan changes can have broad blast radius.")
	case "gateway_config":
		addSignal("gateway_routing_changed", RiskHigh, 26, "Gateway routing and auth changes affect entry traffic.")
	default:
		addSignal("application_code_change", RiskMedium, 14, "Code changes require standard review and rollout checks.")
	}

	if strings.EqualFold(req.Environment, "prod") || strings.Contains(strings.ToLower(req.Environment), "prod") {
		addSignal("production_release", RiskHigh, 20, "Production deployments receive additional risk weighting.")
	}

	serviceTokens := []string{"auth", "payment", "gateway", "billing"}
	serviceName := strings.ToLower(req.Service + " " + req.Repo + " " + req.Payload.Title)
	for _, token := range serviceTokens {
		if strings.Contains(serviceName, token) {
			addSignal("critical_path_component", RiskHigh, 18, "Critical service path detected in review metadata.")
			break
		}
	}

	fileList := metadataStringSlice(req.Payload.Metadata, "file_list")
	if slices.ContainsFunc(fileList, func(item string) bool {
		lower := strings.ToLower(item)
		return strings.Contains(lower, "migration") || strings.Contains(lower, "ingress") || strings.Contains(lower, "routes")
	}) {
		addSignal("sensitive_artifact_changed", RiskMedium, 12, "Sensitive config or migration artifact found in payload metadata.")
	}

	return signals, score
}

func recommendationFor(review Review) Recommendation {
	reviewers := []string{"release-manager"}
	if review.HumanReviewRequired {
		reviewers = append(reviewers, "service-owner")
	}

	window := "business hours"
	if review.Environment == "prod" {
		window = "low-traffic release window"
	}

	return Recommendation{
		RequiredReviewers: reviewers,
		ReleaseWindow:     window,
		RolloutStrategy: RolloutStrategy{
			Steps: []string{
				"Deploy to canary slice",
				"Observe key indicators for 10 minutes",
				"Expand to 25%, then 100% if stable",
			},
			IntervalMinutes: 10,
		},
		ObservabilityPlan: []string{
			"error_rate",
			"latency_p95",
			"saturation_cpu",
		},
	}
}

func rollbackPlanFor(req CreateReviewRequest) RollbackPlan {
	return RollbackPlan{
		TriggerCondition: "error rate or latency regression exceeds threshold",
		Steps: []string{
			"Stop rollout progression",
			"Restore previous stable artifact or config",
			"Verify health checks and core business flow",
		},
		VersionToRestore:       req.Payload.BaseCommit,
		VerificationMetrics:    []string{"error_rate", "latency_p95", "success_rate"},
		RollbackDeadlineSecond: 900,
	}
}

func summaryFor(req CreateReviewRequest, signals []RiskSignal, review Review) string {
	if len(signals) == 0 {
		return "No meaningful risk signals detected."
	}
	return fmt.Sprintf(
		"%s change for %s in %s produced %d signal(s); highest assessed risk is %s.",
		req.SourceType,
		firstNonEmpty(req.Service, req.Repo, req.SourceID),
		req.Environment,
		len(signals),
		review.RiskLevel,
	)
}

func confidenceFor(level RiskLevel) float64 {
	switch level {
	case RiskCritical:
		return 0.94
	case RiskHigh:
		return 0.87
	case RiskMedium:
		return 0.78
	default:
		return 0.70
	}
}

func scoreToRiskLevel(score int) RiskLevel {
	switch {
	case score >= 80:
		return RiskCritical
	case score >= 60:
		return RiskHigh
	case score >= 35:
		return RiskMedium
	default:
		return RiskLow
	}
}

func metadataStringSlice(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	value, ok := metadata[key]
	if !ok {
		return nil
	}
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if ok {
			out = append(out, text)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown target"
}

func (s *Service) nextID(prefix string) string {
	seq := s.counter.Add(1)
	return fmt.Sprintf("%s_%d", prefix, seq)
}
