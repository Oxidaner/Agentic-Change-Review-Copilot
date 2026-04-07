package review

import (
	"errors"
	"os"
	"path/filepath"
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

func TestGetReviewReconstructsSuggestedSignalsFromPersistedEvidence(t *testing.T) {
	store := NewMemoryStore()
	service := NewService(store)

	record := Record{
		Review: Review{
			ReviewID: "rvw_reconstructed",
			Status:   StatusWaitingHumanReview,
		},
		TaskID: "tsk_reconstructed",
		Evidence: []EvidenceItem{
			{
				EvidenceID:     "ev_hybrid",
				Type:           "hybrid_analysis",
				Source:         "change_analyzer",
				Title:          "Hybrid analysis result",
				ContentSnippet: "Persisted summary",
				Confidence:     0.83,
				Metadata: map[string]any{
					"mode":                  "rules_plus_llm_skeleton",
					"analyzer":              "persisted_analyzer",
					"requires_human_review": "true",
					"rationale":             []string{"persisted rationale"},
					"suggested_signals": []any{
						map[string]any{
							"signal_name":  "persisted_auth_signal",
							"severity":     string(RiskHigh),
							"explanation":  "Persisted explanation",
						},
					},
				},
			},
		},
	}

	if err := store.Save(record); err != nil {
		t.Fatalf("save record: %v", err)
	}

	got, err := service.GetReview(record.Review.ReviewID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if len(got.Analysis.SuggestedSignals) != 1 {
		t.Fatalf("suggested_signals = %+v, want 1 reconstructed signal", got.Analysis.SuggestedSignals)
	}
	if got.Analysis.SuggestedSignals[0].SignalName != "persisted_auth_signal" {
		t.Fatalf("signal_name = %s, want persisted_auth_signal", got.Analysis.SuggestedSignals[0].SignalName)
	}
}

func TestGetReviewReconstructsSuggestedSignalsAfterAnalysisPointerRemoved(t *testing.T) {
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
		SourceID:    "PR-777",
		Repo:        "gateway-service",
		Service:     "gateway-service",
		Environment: "prod",
		Payload: ReviewPayload{
			Title: "adjust auth routing",
		},
	})
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	record, err := store.Get(resp.ReviewID)
	if err != nil {
		t.Fatalf("get stored record: %v", err)
	}
	record.Analysis = nil
	if err := store.Save(record); err != nil {
		t.Fatalf("resave record without analysis: %v", err)
	}

	got, err := service.GetReview(resp.ReviewID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if len(got.Analysis.SuggestedSignals) != 1 {
		t.Fatalf("suggested_signals = %+v, want analyzer suggestion after reconstruction", got.Analysis.SuggestedSignals)
	}
	if got.Analysis.SuggestedSignals[0].SignalName != "llm_gateway_auth_regression" {
		t.Fatalf("signal_name = %s, want llm_gateway_auth_regression", got.Analysis.SuggestedSignals[0].SignalName)
	}
}

func TestNewService_RemainsHeuristicByDefaultEvenWhenConfigExists(t *testing.T) {
	dir := t.TempDir()
	writeServiceConfigFixture(t, dir, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini"
}`, `{
  "api_key": "sk-local-123"
}`)

	withWorkingDir(t, dir, func() {
		service := NewService(NewMemoryStore())
		if _, ok := service.analyzer.(HeuristicAnalyzer); !ok {
			t.Fatalf("analyzer type = %T, want HeuristicAnalyzer", service.analyzer)
		}
	})
}

func TestNewServiceWithConfig_SelectsAgentAnalyzerWhenConfigValid(t *testing.T) {
	dir := t.TempDir()
	writeServiceConfigFixture(t, dir, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini"
}`, `{
  "api_key": "sk-local-123"
}`)

	withWorkingDir(t, dir, func() {
		service := NewService(NewMemoryStore(), WithAnalyzerFromConfig())
		analyzer, ok := service.analyzer.(agentAnalyzer)
		if !ok {
			t.Fatalf("analyzer type = %T, want agentAnalyzer", service.analyzer)
		}
		if analyzer.runtime == nil {
			t.Fatal("agent analyzer runtime = nil, want real runtime")
		}
		if analyzer.fallback == nil {
			t.Fatal("agent analyzer fallback = nil, want heuristic fallback")
		}
	})
}

func TestNewServiceWithConfig_FallsBackToHeuristicAnalyzerWhenConfigMissing(t *testing.T) {
	withWorkingDir(t, t.TempDir(), func() {
		service := NewService(NewMemoryStore(), WithAnalyzerFromConfig())
		if _, ok := service.analyzer.(HeuristicAnalyzer); !ok {
			t.Fatalf("analyzer type = %T, want HeuristicAnalyzer", service.analyzer)
		}
	})
}

func TestNewServiceWithConfig_FallsBackToHeuristicAnalyzerWhenConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	writeServiceConfigFixture(t, dir, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini"
}`, "")

	withWorkingDir(t, dir, func() {
		service := NewService(NewMemoryStore(), WithAnalyzerFromConfig())
		if _, ok := service.analyzer.(HeuristicAnalyzer); !ok {
			t.Fatalf("analyzer type = %T, want HeuristicAnalyzer", service.analyzer)
		}
	})
}

func TestWithAnalyzer_OverridesDefaultConstructorBehavior(t *testing.T) {
	dir := t.TempDir()
	writeServiceConfigFixture(t, dir, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini"
}`, `{
  "api_key": "sk-local-123"
}`)

	override := stubAnalyzer{
		analysis: HybridAnalysis{
			Analyzer: "override",
			Summary:  "override",
		},
	}

	withWorkingDir(t, dir, func() {
		service := NewService(NewMemoryStore(), WithAnalyzerFromConfig(), WithAnalyzer(override))
		if got, ok := service.analyzer.(stubAnalyzer); !ok {
			t.Fatalf("analyzer type = %T, want stubAnalyzer override", service.analyzer)
		} else if got.analysis.Analyzer != "override" {
			t.Fatalf("override analyzer = %+v, want stub override", got)
		}
	})
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

func writeServiceConfigFixture(t *testing.T, root, publicBody, localBody string) {
	t.Helper()

	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "review-agent.json"), []byte(publicBody), 0o600); err != nil {
		t.Fatalf("write public config: %v", err)
	}
	if localBody != "" {
		if err := os.WriteFile(filepath.Join(configDir, "review-agent.local.json"), []byte(localBody), 0o600); err != nil {
			t.Fatalf("write local config: %v", err)
		}
	}
}

func withWorkingDir(t *testing.T, dir string, fn func()) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working dir: %v", err)
		}
	})

	fn()
}
