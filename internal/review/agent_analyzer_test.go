package review

import (
	"errors"
	"testing"
)

func TestAgentAnalyzer_Analyze_ReturnsRuntimeResultWithoutFallback(t *testing.T) {
	runtime := stubAnalyzer{
		analysis: HybridAnalysis{
			Mode:                "agent_runtime",
			Analyzer:            "openai_compatible",
			Summary:             "Runtime completed analysis.",
			Confidence:          0.93,
			RequiresHumanReview: true,
			Rationale:           []string{"runtime signal"},
			SuggestedSignals: []SuggestedRiskSignal{
				{
					SignalName:  "runtime_signal",
					Severity:    RiskHigh,
					Explanation: "returned by runtime",
				},
			},
		},
	}
	analyzer := newAgentAnalyzer(runtime, HeuristicAnalyzer{})

	got, err := analyzer.Analyze(agentRuntimeTestInput())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got.Analyzer != "openai_compatible" {
		t.Fatalf("analyzer = %q, want runtime result", got.Analyzer)
	}
	if got.Mode != "agent_runtime" {
		t.Fatalf("mode = %q, want agent_runtime", got.Mode)
	}
	if got.Summary != "Runtime completed analysis." {
		t.Fatalf("summary = %q, want runtime summary", got.Summary)
	}
	if len(got.SuggestedSignals) != 1 || got.SuggestedSignals[0].SignalName != "runtime_signal" {
		t.Fatalf("suggested_signals = %+v, want runtime signal", got.SuggestedSignals)
	}
}

func TestAgentAnalyzer_Analyze_FallsBackToHeuristicWhenRuntimeFails(t *testing.T) {
	analyzer := newAgentAnalyzer(stubAnalyzer{
		err: errors.New("runtime unavailable"),
	}, HeuristicAnalyzer{})

	got, err := analyzer.Analyze(agentRuntimeTestInput())
	if err != nil {
		t.Fatalf("Analyze() error = %v, want heuristic fallback result", err)
	}
	if got.Analyzer != "agent_runtime_fallback" {
		t.Fatalf("analyzer = %q, want explicit fallback marker", got.Analyzer)
	}
	if got.Mode != "agent_runtime_with_fallback" {
		t.Fatalf("mode = %q, want explicit fallback mode", got.Mode)
	}
	if got.Summary == "" {
		t.Fatal("summary = empty, want heuristic fallback summary")
	}
	if len(got.Rationale) == 0 {
		t.Fatal("rationale = empty, want fallback rationale")
	}
	if got.Rationale[0] != "agent runtime failed: runtime unavailable" {
		t.Fatalf("first rationale = %q, want runtime failure marker", got.Rationale[0])
	}
}

func TestAgentAnalyzer_Analyze_ReturnsCombinedErrorWhenRuntimeAndFallbackFail(t *testing.T) {
	analyzer := newAgentAnalyzer(stubAnalyzer{
		err: errors.New("runtime unavailable"),
	}, stubAnalyzer{
		err: errors.New("fallback unavailable"),
	})

	_, err := analyzer.Analyze(agentRuntimeTestInput())
	if err == nil {
		t.Fatal("Analyze() error = nil, want combined runtime and fallback error")
	}
	if got := err.Error(); got != "agent runtime failed: runtime unavailable; fallback analysis failed: fallback unavailable" {
		t.Fatalf("error = %q, want combined failure context", got)
	}
}
