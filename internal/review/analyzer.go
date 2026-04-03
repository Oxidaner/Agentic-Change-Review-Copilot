package review

import "fmt"

// AnalyzerInput is the structured payload passed into the model-assisted
// analysis stage.
type AnalyzerInput struct {
	Bundle        ChangeBundle
	Understanding ChangeUnderstanding
	Evidence      []EvidenceItem
}

// ChangeAnalyzer is the seam where a future LLM + tool-calling implementation
// can be integrated without rewriting the rest of the review workflow.
type ChangeAnalyzer interface {
	Analyze(input AnalyzerInput) (HybridAnalysis, error)
}

// HeuristicAnalyzer is the default fallback analyzer used until a real model
// adapter is wired into the runtime.
type HeuristicAnalyzer struct{}

func (HeuristicAnalyzer) Analyze(input AnalyzerInput) (HybridAnalysis, error) {
	rationale := make([]string, 0, 4)
	rationale = append(rationale, input.Understanding.RiskHints...)

	suggestedSignals := make([]SuggestedRiskSignal, 0, 3)
	if containsAuthScope(input.Bundle) {
		suggestedSignals = append(suggestedSignals, SuggestedRiskSignal{
			SignalName:  "llm_auth_path_concern",
			Severity:    RiskHigh,
			Explanation: "Analyzer flagged auth-related scope as requiring stronger release scrutiny.",
		})
		rationale = append(rationale, "auth-related scope raises release sensitivity")
	}
	if containsSensitiveArtifact(input.Bundle.FileList) {
		suggestedSignals = append(suggestedSignals, SuggestedRiskSignal{
			SignalName:  "llm_sensitive_artifact_concern",
			Severity:    RiskMedium,
			Explanation: "Analyzer flagged sensitive artifact changes as rollout-sensitive.",
		})
		rationale = append(rationale, "sensitive artifact changes increase rollback complexity")
	}
	if input.Bundle.Environment == "prod" {
		rationale = append(rationale, "production environment raises operational impact")
	}

	return HybridAnalysis{
		Mode:                "rules_plus_llm_skeleton",
		Analyzer:            "heuristic_fallback",
		Summary:             fmt.Sprintf("Hybrid analyzer assessed %s for %s in %s.", firstNonEmpty(input.Bundle.SourceType, "change"), firstNonEmpty(input.Bundle.Service, input.Bundle.Repo, input.Bundle.ChangeID), firstNonEmpty(input.Bundle.Environment, "unknown environment")),
		Confidence:          heuristicAnalysisConfidence(input.Bundle, input.Understanding),
		RequiresHumanReview: input.Bundle.Environment == "prod" || containsAuthScope(input.Bundle),
		Rationale:           dedupeStrings(rationale),
		SuggestedSignals:    dedupeSuggestedSignals(suggestedSignals),
	}, nil
}

func heuristicAnalysisConfidence(bundle ChangeBundle, understanding ChangeUnderstanding) float64 {
	confidence := 0.62
	if len(understanding.RiskHints) > 0 {
		confidence += 0.08
	}
	if len(bundle.FileList) > 0 {
		confidence += 0.05
	}
	if containsCriticalPath(bundle) {
		confidence += 0.08
	}
	if confidence > 0.9 {
		confidence = 0.9
	}
	return confidence
}

func dedupeSuggestedSignals(signals []SuggestedRiskSignal) []SuggestedRiskSignal {
	if len(signals) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(signals))
	out := make([]SuggestedRiskSignal, 0, len(signals))
	for _, signal := range signals {
		if signal.SignalName == "" {
			continue
		}
		if _, ok := seen[signal.SignalName]; ok {
			continue
		}
		seen[signal.SignalName] = struct{}{}
		out = append(out, signal)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
