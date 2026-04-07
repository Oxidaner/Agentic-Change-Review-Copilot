package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type chatCompletionClient interface {
	CreateCompletion(req ChatCompletionRequest) (ChatCompletionResponse, error)
}

type AgentRuntime struct {
	cfg      AgentConfig
	client   chatCompletionClient
	registry agentToolRegistry
}

func NewAgentRuntime(cfg AgentConfig, client chatCompletionClient) *AgentRuntime {
	return &AgentRuntime{
		cfg:      cfg,
		client:   client,
		registry: newAgentToolRegistry(),
	}
}

func (r *AgentRuntime) Analyze(input AnalyzerInput) (HybridAnalysis, error) {
	if r == nil {
		return HybridAnalysis{}, errors.New("agent runtime is nil")
	}
	if r.client == nil {
		return HybridAnalysis{}, errors.New("chat completions client is nil")
	}

	maxRounds := r.cfg.MaxRounds
	if maxRounds <= 0 {
		maxRounds = 3
	}
	maxToolCalls := r.cfg.MaxToolCallsPerRound
	if maxToolCalls <= 0 {
		maxToolCalls = 4
	}

	messages := []ChatMessage{
		{
			Role: "system",
			Content: strings.TrimSpace(`You are a release review agent.
Use only the provided read-only tools.
Return the final result as JSON matching HybridAnalysis fields:
summary, confidence, requires_human_review, rationale, suggested_signals.`),
		},
		{
			Role:    "user",
			Content: buildAgentRuntimeUserPrompt(input),
		},
	}

	for round := 1; round <= maxRounds; round++ {
		req := ChatCompletionRequest{
			Model:    r.cfg.Model,
			Messages: messages,
		}
		if r.cfg.ToolCallingEnabled {
			req.Tools = r.registry.definitions()
		}

		resp, err := r.client.CreateCompletion(req)
		if err != nil {
			return HybridAnalysis{}, err
		}
		if len(resp.Choices) == 0 {
			return HybridAnalysis{}, errors.New("chat completion returned no choices")
		}

		message := resp.Choices[0].Message
		messages = append(messages, message)

		if len(message.ToolCalls) == 0 {
			analysis, err := parseRuntimeHybridAnalysis(message.Content)
			if err != nil {
				return HybridAnalysis{}, err
			}
			return analysis, nil
		}
		if !r.cfg.ToolCallingEnabled {
			return HybridAnalysis{}, errors.New("tool calls returned while tool calling is disabled")
		}

		if len(message.ToolCalls) > maxToolCalls {
			return HybridAnalysis{}, fmt.Errorf("max tool calls per round exceeded: %d > %d", len(message.ToolCalls), maxToolCalls)
		}

		for _, call := range message.ToolCalls {
			if err := validateToolCall(call); err != nil {
				return HybridAnalysis{}, err
			}
			toolMessage, err := r.registry.execute(input, call)
			if err != nil {
				return HybridAnalysis{}, err
			}
			messages = append(messages, toolMessage)
		}

		if round == maxRounds {
			return HybridAnalysis{}, fmt.Errorf("max rounds exceeded: %d", maxRounds)
		}
	}

	return HybridAnalysis{}, fmt.Errorf("max rounds exceeded: %d", maxRounds)
}

func buildAgentRuntimeUserPrompt(input AnalyzerInput) string {
	return fmt.Sprintf(
		"Analyze change %s for service %s in environment %s. Use tools for details when needed and avoid speculation.",
		firstNonEmpty(input.Bundle.ChangeID, "unknown-change"),
		firstNonEmpty(input.Bundle.Service, input.Bundle.Repo, "unknown-service"),
		firstNonEmpty(input.Bundle.Environment, "unknown-environment"),
	)
}

func parseRuntimeHybridAnalysis(content string) (HybridAnalysis, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return HybridAnalysis{}, errors.New("assistant returned empty final analysis")
	}

	var analysis HybridAnalysis
	if err := json.Unmarshal([]byte(content), &analysis); err != nil {
		return HybridAnalysis{}, fmt.Errorf("parse final analysis json: %w", err)
	}
	if analysis.Summary == "" {
		return HybridAnalysis{}, errors.New("final analysis missing summary")
	}
	analysis.SuggestedSignals = normalizeSuggestedRiskSignals(analysis.SuggestedSignals)
	if analysis.Mode == "" {
		analysis.Mode = "agent_runtime"
	}
	if analysis.Analyzer == "" {
		analysis.Analyzer = "openai_compatible"
	}
	return analysis, nil
}

func normalizeSuggestedRiskSignals(signals []SuggestedRiskSignal) []SuggestedRiskSignal {
	if len(signals) == 0 {
		return nil
	}

	out := make([]SuggestedRiskSignal, 0, len(signals))
	for _, signal := range signals {
		signal.SignalName = strings.TrimSpace(signal.SignalName)
		signal.Explanation = strings.TrimSpace(signal.Explanation)
		signal.Severity = normalizeRiskLevel(signal.Severity)
		out = append(out, signal)
	}
	return out
}

func normalizeRiskLevel(level RiskLevel) RiskLevel {
	switch RiskLevel(strings.ToUpper(strings.TrimSpace(string(level)))) {
	case RiskCritical:
		return RiskCritical
	case RiskHigh:
		return RiskHigh
	case RiskMedium:
		return RiskMedium
	case RiskLow:
		return RiskLow
	default:
		return RiskLow
	}
}

func validateToolCall(call ToolCall) error {
	if strings.TrimSpace(call.ID) == "" {
		return errors.New("tool call missing id")
	}
	if call.Type != "function" {
		return fmt.Errorf("unsupported tool call type: %q", call.Type)
	}
	return nil
}
