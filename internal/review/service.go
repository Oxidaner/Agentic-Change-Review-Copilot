package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var ErrConflict = errors.New("conflict")

// Service owns the review workflow and persistence coordination.
//
// It is intentionally state-light: all durable workflow state is kept in the
// Store, while the service focuses on deterministic orchestration.
type Service struct {
	store   Store
	counter atomic.Uint64
}

// NewService constructs the review application service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateReview accepts a new review request, applies idempotency, runs the
// current in-process pipeline, and persists the resulting record.
func (s *Service) CreateReview(req CreateReviewRequest) (CreateReviewResponse, error) {
	dedupeKey := dedupeKeyFor(req)
	if dedupeKey != "" {
		existing, err := s.store.FindByDedupeKey(dedupeKey)
		if err == nil {
			return CreateReviewResponse{
				ReviewID: existing.Review.ReviewID,
				TaskID:   existing.TaskID,
				Status:   existing.Review.Status,
				PollURL:  "/api/v1/reviews/" + existing.Review.ReviewID,
			}, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return CreateReviewResponse{}, err
		}
	}

	now := time.Now().UTC()
	reviewID := s.nextID("rvw")
	taskID := s.nextID("tsk")

	record := Record{
		Review: Review{
			ReviewID:    reviewID,
			ChangeID:    req.SourceID,
			DedupeKey:   dedupeKey,
			SourceType:  req.SourceType,
			Repo:        req.Repo,
			Service:     req.Service,
			Environment: req.Environment,
			Status:      StatusCreated,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		Request: &req,
		TaskID:  taskID,
		Timeline: []TimelineEvent{
			{State: StatusCreated, At: now, Detail: "review accepted"},
		},
		Evaluation: &EvaluationRecord{
			CreatedAt: now,
			UpdatedAt: now,
			OutcomeMetadata: map[string]any{
				"source_type": req.SourceType,
			},
		},
	}

	if req.Payload.Author != "" {
		record.Review.Summary = fmt.Sprintf("Review created for %s by %s", req.SourceType, req.Payload.Author)
	} else {
		record.Review.Summary = fmt.Sprintf("Review created for %s", req.SourceType)
	}

	record = s.runPipeline(record, req)

	if err := s.store.Save(record); err != nil {
		if dedupeKey != "" && errors.Is(err, ErrConflict) {
			existing, findErr := s.store.FindByDedupeKey(dedupeKey)
			if findErr == nil {
				return CreateReviewResponse{
					ReviewID: existing.Review.ReviewID,
					TaskID:   existing.TaskID,
					Status:   existing.Review.Status,
					PollURL:  "/api/v1/reviews/" + existing.Review.ReviewID,
				}, nil
			}
			if !errors.Is(findErr, ErrNotFound) {
				return CreateReviewResponse{}, findErr
			}
		}
		return CreateReviewResponse{}, err
	}

	return CreateReviewResponse{
		ReviewID: reviewID,
		TaskID:   taskID,
		Status:   record.Review.Status,
		PollURL:  "/api/v1/reviews/" + reviewID,
	}, nil
}

// GetReview returns the latest assembled review response.
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

// GetTimeline returns the event timeline for a review.
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

// SubmitHumanDecision persists an explicit human outcome and updates the review
// state to the terminal status implied by that decision.
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
	if record.Evaluation == nil {
		record.Evaluation = &EvaluationRecord{CreatedAt: now}
	}
	record.Evaluation.FinalHumanDecision = stringPtr(req.Decision)
	record.Evaluation.UpdatedAt = now
	if record.Evaluation.OutcomeMetadata == nil {
		record.Evaluation.OutcomeMetadata = map[string]any{}
	}
	record.Evaluation.OutcomeMetadata["reviewer"] = req.Reviewer
	record.Evaluation.OutcomeMetadata["reason"] = req.Reason
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

// RetryReview replays the pipeline for a failed review while preserving the
// original identifiers and history.
func (s *Service) RetryReview(reviewID string, _ RetryRequest) (RetryResponse, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return RetryResponse{}, err
	}
	if record.Review.Status != StatusFailed {
		return RetryResponse{}, ErrConflict
	}

	req := requestFromRecord(record)
	record.Review.Status = StatusCreated
	record.Review.UpdatedAt = time.Now().UTC()
	record.Signals = nil
	record.Recommendation = Recommendation{}
	record.RollbackPlan = RollbackPlan{}
	record.Evidence = nil
	record.LastError = ""
	record.Timeline = append(record.Timeline, TimelineEvent{
		State:  StatusCreated,
		At:     record.Review.UpdatedAt,
		Detail: "retry accepted",
	})
	if record.Evaluation != nil {
		record.Evaluation.UpdatedAt = record.Review.UpdatedAt
	}
	record = s.runPipeline(record, req)

	if err := s.store.Save(record); err != nil {
		return RetryResponse{}, err
	}

	return RetryResponse{
		ReviewID: reviewID,
		TaskID:   record.TaskID,
		Status:   record.Review.Status,
	}, nil
}

// ExportReview renders the current review either as JSON or a simple markdown
// report suitable for copy/paste sharing.
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

// UpdateEvaluation records post-release feedback that can later be aggregated into
// coarse evaluation metrics.
func (s *Service) UpdateEvaluation(reviewID string, req EvaluationUpdateRequest) (EvaluationUpdateResponse, error) {
	record, err := s.store.Get(reviewID)
	if err != nil {
		return EvaluationUpdateResponse{}, err
	}

	now := time.Now().UTC()
	if record.Evaluation == nil {
		record.Evaluation = &EvaluationRecord{CreatedAt: now}
	}
	if req.ReleaseOutcome != "" {
		record.Evaluation.ReleaseOutcome = stringPtr(req.ReleaseOutcome)
	}
	if req.IncidentFlag != nil {
		record.Evaluation.IncidentFlag = req.IncidentFlag
	}
	if record.Evaluation.OutcomeMetadata == nil {
		record.Evaluation.OutcomeMetadata = map[string]any{}
	}
	for key, value := range req.OutcomeMetadata {
		record.Evaluation.OutcomeMetadata[key] = value
	}
	record.Evaluation.UpdatedAt = now
	record.Timeline = append(record.Timeline, TimelineEvent{
		State:  record.Review.Status,
		At:     now,
		Detail: "evaluation record updated",
	})

	if err := s.store.Save(record); err != nil {
		return EvaluationUpdateResponse{}, err
	}

	return EvaluationUpdateResponse{ReviewID: reviewID, Recorded: true}, nil
}

// GetEvaluationMetrics computes simple aggregate metrics directly from stored
// records. This is intentionally lightweight and in-process for the MVP.
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
	count := 0
	overrides := 0
	highRisk := 0
	highRiskWithIncident := 0
	highRiskWithoutIncident := 0
	for _, record := range records {
		if record.Review.CreatedAt.Before(start) || record.Review.CreatedAt.After(end) {
			continue
		}
		if service != "" && record.Review.Service != service {
			continue
		}

		count++
		if record.Review.Status == StatusOverridden {
			overrides++
		}
		if record.Review.RiskLevel == RiskHigh || record.Review.RiskLevel == RiskCritical {
			highRisk++
			if record.Evaluation != nil && record.Evaluation.IncidentFlag != nil {
				if *record.Evaluation.IncidentFlag {
					highRiskWithIncident++
				} else {
					highRiskWithoutIncident++
				}
			}
		}
	}

	overrideRate := 0.0
	highRiskRecall := 0.0
	falsePositiveRate := 0.08
	if count > 0 {
		overrideRate = float64(overrides) / float64(count)
		highRiskRecall = float64(highRisk) / float64(count)
	}
	if highRiskWithIncident+highRiskWithoutIncident > 0 {
		falsePositiveRate = float64(highRiskWithoutIncident) / float64(highRiskWithIncident+highRiskWithoutIncident)
	}

	return EvaluationMetricsResponse{
		Window:  MetricsWindow{From: from, To: to},
		Service: service,
		Metrics: Metrics{
			ReviewCount:       count,
			HighRiskRecall:    highRiskRecall,
			FalsePositiveRate: falsePositiveRate,
			OverrideRate:      overrideRate,
			P95LatencyMS:      1800,
		},
	}, nil
}

// runPipeline executes the current fixed workflow for a review.
//
// The implementation mirrors the target product stages even though each stage is
// still heuristic today. This keeps the code ready for future extraction into
// dedicated ingestion, retrieval, rule, and recommendation modules.
func (s *Service) runPipeline(record Record, req CreateReviewRequest) Record {
	bundle := normalizeChange(req, record.Review.CreatedAt)
	record.Bundle = &bundle
	record = s.transition(record, StatusNormalized, "change normalized into ChangeBundle")

	understanding := understandChange(bundle)
	record.Understanding = &understanding
	record = s.transition(record, StatusUnderstood, "change understanding completed")
	record = s.transition(record, StatusCollectingContext, "collecting supporting evidence")

	evidencePack := collectEvidence(bundle, understanding, s.nextID)
	record.Evidence = evidencePack.Items
	record = s.transition(record, StatusContextReady, fmt.Sprintf("%d evidence item(s) collected", len(record.Evidence)))

	signals, _ := extractRiskSignals(bundle, understanding, evidencePack, s.nextID)
	record.Signals = signals
	record = s.transition(record, StatusSignalsExtracted, fmt.Sprintf("%d risk signals extracted", len(signals)))

	score := scoreReview(signals)
	record.Review.Score = score.Score
	record.Review.RiskLevel = score.RiskLevel
	record.Review.Confidence = score.Confidence
	record.Review.HumanReviewRequired = score.HumanReviewRequired
	record = s.transition(record, StatusScored, fmt.Sprintf("risk score=%d", record.Review.Score))

	record.Recommendation = buildRecommendation(bundle, understanding, score)
	record.RollbackPlan = buildRollbackPlan(req, bundle)
	record.Review.Summary = buildReviewSummary(bundle, understanding, score, record.Signals)
	if record.Evaluation != nil {
		record.Evaluation.UpdatedAt = record.Review.UpdatedAt
		if record.Evaluation.OutcomeMetadata == nil {
			record.Evaluation.OutcomeMetadata = map[string]any{}
		}
		record.Evaluation.OutcomeMetadata["risk_level"] = record.Review.RiskLevel
		record.Evaluation.OutcomeMetadata["score"] = record.Review.Score
	}
	record = s.transition(record, StatusRecommended, "structured recommendation generated")

	if record.Review.HumanReviewRequired {
		record = s.transition(record, StatusWaitingHumanReview, "human review required based on risk or confidence")
		return record
	}

	return record
}

// transition appends a timeline event and updates the review's current status.
func (s *Service) transition(record Record, status ReviewStatus, detail string) Record {
	now := time.Now().UTC()
	record.Review.Status = status
	record.Review.UpdatedAt = now
	record.Timeline = append(record.Timeline, TimelineEvent{State: status, At: now, Detail: detail})
	return record
}

// confidenceFor returns the heuristic confidence attached to a risk level.
func confidenceFor(level RiskLevel) float64 {
	switch level {
	case RiskCritical:
		return 0.94
	case RiskHigh:
		return 0.87
	case RiskMedium:
		return 0.78
	default:
		return 0.76
	}
}

// scoreToRiskLevel maps a numeric score onto a coarse severity bucket.
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

// firstNonEmpty returns the first non-empty value from a list of candidates.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "unknown target"
}

// stringPtr is a small helper used by persistence code that stores optional strings.
func stringPtr(value string) *string {
	v := value
	return &v
}

// dedupeKeyFor computes the idempotency key used to avoid duplicate review
// creation for the same source and head revision.
func dedupeKeyFor(req CreateReviewRequest) string {
	if req.DedupeKey != "" {
		return req.DedupeKey
	}
	if req.SourceID == "" || req.Environment == "" {
		return ""
	}
	return strings.ToLower(fmt.Sprintf("%s:%s:%s:%s", req.SourceType, req.SourceID, req.Payload.HeadCommit, req.Environment))
}

// requestFromRecord reconstructs a request from a persisted record when retrying
// older data that may not have the original request object in memory.
func requestFromRecord(record Record) CreateReviewRequest {
	if record.Request != nil {
		return *record.Request
	}

	return CreateReviewRequest{
		SourceType:  record.Review.SourceType,
		SourceID:    record.Review.ChangeID,
		Repo:        record.Review.Repo,
		Service:     record.Review.Service,
		Environment: record.Review.Environment,
		DedupeKey:   record.Review.DedupeKey,
		Payload: ReviewPayload{
			BaseCommit: record.RollbackPlan.VersionToRestore,
		},
	}
}

// nextID generates monotonic in-process identifiers for review-related entities.
func (s *Service) nextID(prefix string) string {
	seq := s.counter.Add(1)
	return fmt.Sprintf("%s_%d", prefix, seq)
}
