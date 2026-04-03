package review

import (
	"errors"
	"testing"
	"time"
)

func TestRetryReviewReplaysPipeline(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	req := CreateReviewRequest{
		SourceType:  "pull_request",
		SourceID:    "PR-999",
		Repo:        "gateway-service",
		Service:     "api-gateway",
		Environment: "prod",
		Payload: ReviewPayload{
			Title:      "adjust auth routing",
			Author:     "alice",
			BaseCommit: "abc123",
			HeadCommit: "def456",
			Metadata: map[string]any{
				"file_list": []any{"configs/routes.yaml"},
			},
		},
	}

	record := Record{
		Review: Review{
			ReviewID:    "rvw_failed",
			ChangeID:    req.SourceID,
			DedupeKey:   dedupeKeyFor(req),
			SourceType:  req.SourceType,
			Repo:        req.Repo,
			Service:     req.Service,
			Environment: req.Environment,
			Status:      StatusFailed,
			CreatedAt:   time.Now().UTC().Add(-time.Minute),
			UpdatedAt:   time.Now().UTC(),
		},
		Request: &req,
		TaskID:  "tsk_failed",
		Timeline: []TimelineEvent{
			{State: StatusFailed, At: time.Now().UTC(), Detail: "simulated failure"},
		},
		LastError: "simulated failure",
	}

	if err := store.Save(record); err != nil {
		t.Fatalf("seed failed review: %v", err)
	}

	resp, err := service.RetryReview(record.Review.ReviewID, RetryRequest{})
	if err != nil {
		t.Fatalf("retry review: %v", err)
	}

	if resp.Status != StatusWaitingHumanReview {
		t.Fatalf("retry status = %s, want %s", resp.Status, StatusWaitingHumanReview)
	}

	got, err := store.Get(record.Review.ReviewID)
	if err != nil {
		t.Fatalf("load retried review: %v", err)
	}
	if got.Review.Score == 0 {
		t.Fatal("expected retry to regenerate score")
	}
	if len(got.Signals) == 0 {
		t.Fatal("expected retry to regenerate signals")
	}
	if got.LastError != "" {
		t.Fatalf("last_error = %q, want empty", got.LastError)
	}
}

func TestCreateReviewReturnsExistingOnConflict(t *testing.T) {
	existing := Record{
		Review: Review{
			ReviewID:    "rvw_existing",
			ChangeID:    "PR-123",
			DedupeKey:   "github:gateway-service:pr-123:def456:prod",
			SourceType:  "pull_request",
			Repo:        "gateway-service",
			Service:     "api-gateway",
			Environment: "prod",
			Status:      StatusWaitingHumanReview,
		},
		TaskID: "tsk_existing",
	}

	store := &conflictStore{existing: existing}
	service := NewService(store)

	resp, err := service.CreateReview(CreateReviewRequest{
		SourceType:  "pull_request",
		SourceID:    "PR-123",
		Repo:        "gateway-service",
		Service:     "api-gateway",
		Environment: "prod",
		DedupeKey:   existing.Review.DedupeKey,
		Payload: ReviewPayload{
			HeadCommit: "def456",
		},
	})
	if err != nil {
		t.Fatalf("create review: %v", err)
	}
	if resp.ReviewID != existing.Review.ReviewID {
		t.Fatalf("review_id = %s, want %s", resp.ReviewID, existing.Review.ReviewID)
	}
	if resp.TaskID != existing.TaskID {
		t.Fatalf("task_id = %s, want %s", resp.TaskID, existing.TaskID)
	}
}

type conflictStore struct {
	existing Record
}

func (s *conflictStore) Save(Record) error {
	return ErrConflict
}

func (s *conflictStore) Get(reviewID string) (Record, error) {
	if reviewID == s.existing.Review.ReviewID {
		return s.existing, nil
	}
	return Record{}, ErrNotFound
}

func (s *conflictStore) FindByDedupeKey(dedupeKey string) (Record, error) {
	if dedupeKey == s.existing.Review.DedupeKey {
		return s.existing, nil
	}
	return Record{}, ErrNotFound
}

func (s *conflictStore) List() []Record {
	if s.existing.Review.ReviewID == "" {
		return nil
	}
	return []Record{s.existing}
}

var _ Store = (*conflictStore)(nil)

func TestCreateReviewHybridAnalysisMergesAnalyzerSignals(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store, WithAnalyzer(stubAnalyzer{
		analysis: HybridAnalysis{
			Mode:                "rules_plus_llm_skeleton",
			Analyzer:            "stub_llm",
			Summary:             "Model flagged rollout-sensitive auth change.",
			Confidence:          0.91,
			RequiresHumanReview: true,
			Rationale:           []string{"auth path touches production gateway"},
			SuggestedSignals: []SuggestedRiskSignal{
				{
					SignalName:  "llm_gateway_auth_regression",
					Severity:    RiskHigh,
					Explanation: "Model detected gateway auth regression risk.",
				},
			},
		},
	}))

	resp, err := service.CreateReview(CreateReviewRequest{
		SourceType:  "pull_request",
		SourceID:    "PR-321",
		Repo:        "gateway-service",
		Service:     "gateway-service",
		Environment: "prod",
		Payload: ReviewPayload{
			Title:      "adjust auth routing",
			Author:     "alice",
			BaseCommit: "abc123",
			HeadCommit: "def456",
		},
	})
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	got, err := service.GetReview(resp.ReviewID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if got.Analysis.Analyzer != "stub_llm" {
		t.Fatalf("analysis analyzer = %s, want stub_llm", got.Analysis.Analyzer)
	}
	if got.Analysis.Confidence != 0.91 {
		t.Fatalf("analysis confidence = %v, want 0.91", got.Analysis.Confidence)
	}
	found := false
	for _, signal := range got.Signals {
		if signal.SignalName == "llm_gateway_auth_regression" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("signals = %+v, want analyzer-suggested signal", got.Signals)
	}
}

func TestCreateReviewHybridAnalysisFallsBackOnAnalyzerError(t *testing.T) {
	service := NewService(NewMemoryStore(), WithAnalyzer(stubAnalyzer{
		err: errors.New("model unavailable"),
	}))

	resp, err := service.CreateReview(CreateReviewRequest{
		SourceType:  "pull_request",
		SourceID:    "PR-654",
		Repo:        "gateway-service",
		Service:     "gateway-service",
		Environment: "staging",
		Payload: ReviewPayload{
			Title: "adjust auth routing",
		},
	})
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	got, err := service.GetReview(resp.ReviewID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if got.Analysis.Analyzer != "analysis_error_fallback" {
		t.Fatalf("analysis analyzer = %s, want analysis_error_fallback", got.Analysis.Analyzer)
	}
}

type stubAnalyzer struct {
	analysis HybridAnalysis
	err      error
}

func (s stubAnalyzer) Analyze(AnalyzerInput) (HybridAnalysis, error) {
	if s.err != nil {
		return HybridAnalysis{}, s.err
	}
	return s.analysis, nil
}
