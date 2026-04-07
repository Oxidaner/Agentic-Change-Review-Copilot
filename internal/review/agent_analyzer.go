package review

import "fmt"

type agentAnalyzer struct {
	runtime  ChangeAnalyzer
	fallback ChangeAnalyzer
}

func newAgentAnalyzer(runtime ChangeAnalyzer, fallback ChangeAnalyzer) ChangeAnalyzer {
	if runtime == nil {
		return fallbackAnalyzerOrDefault(fallback)
	}
	return agentAnalyzer{
		runtime:  runtime,
		fallback: fallbackAnalyzerOrDefault(fallback),
	}
}

func (a agentAnalyzer) Analyze(input AnalyzerInput) (HybridAnalysis, error) {
	if a.runtime == nil {
		return a.fallback.Analyze(input)
	}

	analysis, err := a.runtime.Analyze(input)
	if err == nil {
		return analysis, nil
	}

	fallback, fallbackErr := a.fallback.Analyze(input)
	if fallbackErr != nil {
		return HybridAnalysis{}, fmt.Errorf("agent runtime failed: %v; fallback analysis failed: %w", err, fallbackErr)
	}

	fallback.Mode = "agent_runtime_with_fallback"
	fallback.Analyzer = "agent_runtime_fallback"
	fallback.Rationale = append([]string{fmt.Sprintf("agent runtime failed: %v", err)}, fallback.Rationale...)
	return fallback, nil
}

func fallbackAnalyzerOrDefault(fallback ChangeAnalyzer) ChangeAnalyzer {
	if fallback == nil {
		return HeuristicAnalyzer{}
	}
	return fallback
}
