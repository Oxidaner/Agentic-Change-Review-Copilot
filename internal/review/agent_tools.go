package review

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type agentToolRegistry struct {
	tools map[string]agentTool
}

type agentTool struct {
	definition LLMTool
	execute    func(AnalyzerInput, string) (string, error)
}

func newAgentToolRegistry() agentToolRegistry {
	registry := agentToolRegistry{
		tools: map[string]agentTool{},
	}

	registry.register(agentTool{
		definition: LLMTool{
			Type: "function",
			Function: LLMToolFunction{
				Name:        "get_change_bundle",
				Description: "Return the normalized change bundle for the current review.",
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties":           map[string]any{},
				},
			},
		},
		execute: func(input AnalyzerInput, _ string) (string, error) {
			return marshalToolResult(changeBundleToolView{
				ChangeID:           input.Bundle.ChangeID,
				SourceType:         input.Bundle.SourceType,
				ChangeType:         input.Bundle.ChangeType,
				Repo:               input.Bundle.Repo,
				Service:            input.Bundle.Service,
				Environment:        input.Bundle.Environment,
				Author:             input.Bundle.Author,
				SubmitTime:         input.Bundle.SubmitTime,
				Title:              input.Bundle.Title,
				FileList:           input.Bundle.FileList,
				ImpactedComponents: input.Bundle.ImpactedComponents,
				DiffSummary:        input.Bundle.DiffSummary,
				ReleaseTarget:      input.Bundle.ReleaseTarget,
				Metadata:           sanitizeChangeBundleToolMetadata(input.Bundle.Metadata),
			})
		},
	})

	registry.register(agentTool{
		definition: LLMTool{
			Type: "function",
			Function: LLMToolFunction{
				Name:        "get_change_understanding",
				Description: "Return the deterministic change understanding derived from the current review context.",
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties":           map[string]any{},
				},
			},
		},
		execute: func(input AnalyzerInput, _ string) (string, error) {
			return marshalToolResult(changeUnderstandingToolView{
				Summary:               input.Understanding.Summary,
				SemanticTags:          input.Understanding.SemanticTags,
				RiskHints:             input.Understanding.RiskHints,
				ImpactedServices:      input.Understanding.ImpactedServices,
				PotentialFailureModes: input.Understanding.PotentialFailureModes,
			})
		},
	})

	registry.register(agentTool{
		definition: LLMTool{
			Type: "function",
			Function: LLMToolFunction{
				Name:        "list_evidence",
				Description: "List all evidence items already collected for the current review.",
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties":           map[string]any{},
				},
			},
		},
		execute: func(input AnalyzerInput, _ string) (string, error) {
			return marshalToolResult(input.Evidence)
		},
	})

	registry.register(agentTool{
		definition: LLMTool{
			Type: "function",
			Function: LLMToolFunction{
				Name:        "get_evidence_by_id",
				Description: "Return a single evidence item by evidence_id.",
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"evidence_id"},
					"properties": map[string]any{
						"evidence_id": map[string]any{
							"type":        "string",
							"description": "The evidence_id to fetch from the current review context.",
						},
					},
				},
			},
		},
		execute: func(input AnalyzerInput, arguments string) (string, error) {
			var args struct {
				EvidenceID string `json:"evidence_id"`
			}
			if err := decodeToolArguments(arguments, &args); err != nil {
				return "", err
			}
			args.EvidenceID = strings.TrimSpace(args.EvidenceID)
			if args.EvidenceID == "" {
				return "", fmt.Errorf("missing required evidence_id")
			}
			for _, item := range input.Evidence {
				if item.EvidenceID == args.EvidenceID {
					return marshalToolResult(item)
				}
			}
			return "", fmt.Errorf("evidence not found: %s", args.EvidenceID)
		},
	})

	registry.register(agentTool{
		definition: LLMTool{
			Type: "function",
			Function: LLMToolFunction{
				Name:        "list_rule_signals",
				Description: "List deterministic rule-derived risk signals for the current review context.",
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties":           map[string]any{},
				},
			},
		},
		execute: func(input AnalyzerInput, _ string) (string, error) {
			signals, _ := extractRiskSignals(
				input.Bundle,
				input.Understanding,
				EvidencePack{Items: input.Evidence},
				newDeterministicIDFactory("sig"),
			)
			return marshalToolResult(signals)
		},
	})

	return registry
}

func (r agentToolRegistry) register(tool agentTool) {
	r.tools[tool.definition.Function.Name] = tool
}

func (r agentToolRegistry) definitions() []LLMTool {
	names := []string{
		"get_change_bundle",
		"get_change_understanding",
		"list_evidence",
		"get_evidence_by_id",
		"list_rule_signals",
	}
	out := make([]LLMTool, 0, len(names))
	for _, name := range names {
		tool, ok := r.tools[name]
		if !ok {
			continue
		}
		out = append(out, tool.definition)
	}
	return out
}

func (r agentToolRegistry) execute(input AnalyzerInput, call ToolCall) (ChatMessage, error) {
	tool, ok := r.tools[call.Function.Name]
	if !ok {
		return ChatMessage{}, fmt.Errorf("unsupported tool: %s", call.Function.Name)
	}

	content, err := tool.execute(input, call.Function.Arguments)
	if err != nil {
		return ChatMessage{}, fmt.Errorf("execute tool %s: %w", call.Function.Name, err)
	}

	return ChatMessage{
		Role:       "tool",
		ToolCallID: call.ID,
		Name:       call.Function.Name,
		Content:    content,
	}, nil
}

func decodeToolArguments(arguments string, out any) error {
	if arguments == "" {
		arguments = "{}"
	}
	decoder := json.NewDecoder(bytes.NewBufferString(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return fmt.Errorf("unexpected fields in tool arguments: %w", err)
		}
		return fmt.Errorf("decode tool arguments: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("decode tool arguments: trailing data")
	}
	return nil
}

func marshalToolResult(v any) (string, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func newDeterministicIDFactory(prefix string) func(string) string {
	var n int
	return func(_ string) string {
		n++
		return fmt.Sprintf("%s-%d", prefix, n)
	}
}

func sanitizeChangeBundleToolMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}

	out := compactMetadataMap(map[string]any{
		"change_window": metadataString(metadata, "change_window"),
		"ticket":        metadataString(metadata, "ticket"),
	})
	if gitMetadata := safeGitEvidenceMetadata(metadata); len(gitMetadata) > 0 {
		if out == nil {
			out = make(map[string]any, 4)
		}
		out["git"] = gitMetadata
	}
	if cmdbMetadata := safeCMDBEvidenceMetadata(metadata); len(cmdbMetadata) > 0 {
		if out == nil {
			out = make(map[string]any, 4)
		}
		out["cmdb"] = cmdbMetadata
	}
	if metricsMetadata := safeMetricsEvidenceMetadata(metadata); len(metricsMetadata) > 0 {
		if out == nil {
			out = make(map[string]any, 4)
		}
		out["metrics"] = metricsMetadata
	}
	if runbookMetadata := safeRunbookEvidenceMetadata(metadata); len(runbookMetadata) > 0 {
		if out == nil {
			out = make(map[string]any, 4)
		}
		out["runbook"] = runbookMetadata
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type changeBundleToolView struct {
	ChangeID           string         `json:"change_id,omitempty"`
	SourceType         string         `json:"source_type,omitempty"`
	ChangeType         string         `json:"change_type,omitempty"`
	Repo               string         `json:"repo,omitempty"`
	Service            string         `json:"service,omitempty"`
	Environment        string         `json:"environment,omitempty"`
	Author             string         `json:"author,omitempty"`
	SubmitTime         any            `json:"submit_time,omitempty"`
	Title              string         `json:"title,omitempty"`
	FileList           []string       `json:"file_list,omitempty"`
	ImpactedComponents []string       `json:"impacted_components,omitempty"`
	DiffSummary        string         `json:"diff_summary,omitempty"`
	ReleaseTarget      string         `json:"release_target,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type changeUnderstandingToolView struct {
	Summary               string   `json:"summary,omitempty"`
	SemanticTags          []string `json:"semantic_tags,omitempty"`
	RiskHints             []string `json:"risk_hints,omitempty"`
	ImpactedServices      []string `json:"impacted_services,omitempty"`
	PotentialFailureModes []string `json:"potential_failure_modes,omitempty"`
}
