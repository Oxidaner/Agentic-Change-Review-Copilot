package review

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestChatCompletionsClient_CreateCompletion(t *testing.T) {
	var gotMethod string
	var gotContentType string
	var gotBody struct {
		Model    string        `json:"model"`
		Messages []ChatMessage `json:"messages"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer local-test-key" {
			t.Fatalf("authorization = %q, want %q", got, "Bearer local-test-key")
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"ok\",\"confidence\":0.9,\"requires_human_review\":false,\"rationale\":[\"clear\"],\"suggested_signals\":[]}"}}]}`))
	}))
	defer server.Close()

	client := NewChatCompletionsClient(AgentConfig{
		BaseURL:   server.URL,
		Model:     "gpt-4.1-mini",
		APIKey:    "local-test-key",
		TimeoutMS: 5000,
	})

	resp, err := client.CreateCompletion(ChatCompletionRequest{
		Model: "gpt-4.1-mini",
		Messages: []ChatMessage{
			{Role: "system", Content: "test"},
		},
	})
	if err != nil {
		t.Fatalf("CreateCompletion() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %s, want %s", gotMethod, http.MethodPost)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content-type = %q, want %q", gotContentType, "application/json")
	}
	if gotBody.Model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want %q", gotBody.Model, "gpt-4.1-mini")
	}
	if len(gotBody.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(gotBody.Messages))
	}
	if gotBody.Messages[0].Role != "system" || gotBody.Messages[0].Content != "test" {
		t.Fatalf("message = %#v, want role system content test", gotBody.Messages[0])
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(resp.Choices))
	}
}

func TestChatCompletionsClient_CreateCompletion_Non2xxIncludesBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"upstream exploded"}}`, http.StatusBadGateway)
	}))
	defer server.Close()

	client := NewChatCompletionsClient(AgentConfig{
		BaseURL:   server.URL,
		Model:     "gpt-4.1-mini",
		APIKey:    "local-test-key",
		TimeoutMS: 5000,
	})

	_, err := client.CreateCompletion(ChatCompletionRequest{
		Model: "gpt-4.1-mini",
		Messages: []ChatMessage{
			{Role: "system", Content: "test"},
		},
	})
	if err == nil {
		t.Fatal("CreateCompletion() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "upstream exploded") {
		t.Fatalf("error = %v, want body text", err)
	}
}

func TestAgentRuntime_Analyze_ToolRoundTripAndFinalJSON(t *testing.T) {
	var requestBodies []ChatCompletionRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requestBodies = append(requestBodies, req)

		w.Header().Set("Content-Type", "application/json")
		switch len(requestBodies) {
		case 1:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"get_change_bundle","arguments":"{}"}}]}}]}`))
		case 2:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"Auth routing change needs staged rollout.\",\"confidence\":0.88,\"requires_human_review\":true,\"rationale\":[\"production release\",\"auth surface changed\"],\"suggested_signals\":[{\"signal_name\":\"llm_auth_path_concern\",\"severity\":\"HIGH\",\"explanation\":\"Auth routing is in scope.\"}]}"}}]}`))
		default:
			t.Fatalf("unexpected request count %d", len(requestBodies))
		}
	}))
	defer server.Close()

	runtime := NewAgentRuntime(
		AgentConfig{
			BaseURL:              server.URL,
			Model:                "gpt-4.1-mini",
			APIKey:               "local-test-key",
			MaxRounds:            2,
			TimeoutMS:            5000,
			ToolCallingEnabled:   true,
			MaxToolCallsPerRound: 2,
		},
		NewChatCompletionsClient(AgentConfig{
			BaseURL:   server.URL,
			Model:     "gpt-4.1-mini",
			APIKey:    "local-test-key",
			TimeoutMS: 5000,
		}),
	)

	got, err := runtime.Analyze(agentRuntimeTestInput())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if got.Summary != "Auth routing change needs staged rollout." {
		t.Fatalf("summary = %q, want parsed final analysis", got.Summary)
	}
	if !got.RequiresHumanReview {
		t.Fatal("requires_human_review = false, want true")
	}
	if len(got.SuggestedSignals) != 1 || got.SuggestedSignals[0].SignalName != "llm_auth_path_concern" {
		t.Fatalf("suggested_signals = %+v, want parsed tool-assisted signal", got.SuggestedSignals)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("requests = %d, want 2", len(requestBodies))
	}
	if len(requestBodies[0].Tools) == 0 {
		t.Fatal("first request tools = 0, want registered tools")
	}
	if len(requestBodies[1].Messages) < 3 {
		t.Fatalf("second request messages = %d, want tool transcript appended", len(requestBodies[1].Messages))
	}
	lastMessage := requestBodies[1].Messages[len(requestBodies[1].Messages)-1]
	if lastMessage.Role != "tool" {
		t.Fatalf("last message role = %q, want tool", lastMessage.Role)
	}
	if lastMessage.ToolCallID != "call-1" {
		t.Fatalf("tool_call_id = %q, want call-1", lastMessage.ToolCallID)
	}
	if !strings.Contains(lastMessage.Content, `"change_id":"octo/gateway-service#42"`) {
		t.Fatalf("tool content = %s, want serialized change bundle", lastMessage.Content)
	}
}

func TestAgentRuntime_Analyze_SanitizesChangeBundleMetadataForToolResponses(t *testing.T) {
	var requestBodies []ChatCompletionRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requestBodies = append(requestBodies, req)

		w.Header().Set("Content-Type", "application/json")
		switch len(requestBodies) {
		case 1:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"get_change_bundle","arguments":"{}"}}]}}]}`))
		case 2:
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"done\",\"confidence\":0.4,\"requires_human_review\":false,\"rationale\":[\"ok\"],\"suggested_signals\":[]}"}}]}`))
		default:
			t.Fatalf("unexpected request count %d", len(requestBodies))
		}
	}))
	defer server.Close()

	runtime := NewAgentRuntime(
		AgentConfig{
			BaseURL:              server.URL,
			Model:                "gpt-4.1-mini",
			APIKey:               "local-test-key",
			MaxRounds:            2,
			TimeoutMS:            5000,
			ToolCallingEnabled:   true,
			MaxToolCallsPerRound: 2,
		},
		NewChatCompletionsClient(AgentConfig{
			BaseURL:   server.URL,
			Model:     "gpt-4.1-mini",
			APIKey:    "local-test-key",
			TimeoutMS: 5000,
		}),
	)

	input := agentRuntimeTestInput()
	input.Bundle.Metadata = map[string]any{
		"git": map[string]any{
			"provider":     "github",
			"repository":   "octo/gateway-service",
			"base_commit":  "abc123",
			"head_commit":  "def456",
			"diff_url":     "https://github.com/octo/gateway-service/pull/42.diff",
			"diff_summary": "adjust auth routing and ingress rules",
			"token":        "secret-token",
		},
		"cmdb": map[string]any{
			"service_name": "gateway-service",
			"service_tier": "tier-1",
			"owner":        "traffic-platform",
			"pagerduty":    "pd-secret",
		},
		"metrics": map[string]any{
			"summary":      "latency_p95 elevated over last 15m",
			"window":       "15m",
			"error_rate":   "0.3%",
			"latency_p95":  "420ms",
			"raw_payload":  "{\"series\":[]}",
			"internal_url": "https://metrics.internal/query",
		},
		"runbook": map[string]any{
			"title":    "Gateway rollback playbook",
			"url":      "https://runbooks.example.com/gateway/rollback",
			"severity": "high",
			"secret":   "do-not-leak",
		},
		"headers": map[string]any{
			"Authorization": "Bearer secret",
		},
	}

	if _, err := runtime.Analyze(input); err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(requestBodies) != 2 {
		t.Fatalf("requests = %d, want 2", len(requestBodies))
	}

	lastMessage := requestBodies[1].Messages[len(requestBodies[1].Messages)-1]
	var toolView changeBundleToolView
	if err := json.Unmarshal([]byte(lastMessage.Content), &toolView); err != nil {
		t.Fatalf("unmarshal tool content: %v", err)
	}

	if toolView.Metadata == nil {
		t.Fatal("tool metadata = nil, want sanitized metadata")
	}
	if _, ok := toolView.Metadata["headers"]; ok {
		t.Fatalf("tool metadata leaked top-level headers: %+v", toolView.Metadata)
	}
	if got := metadataNestedMap(toolView.Metadata, "git"); got["token"] != nil {
		t.Fatalf("tool metadata leaked git token: %+v", got)
	}
	if got := metadataNestedMap(toolView.Metadata, "cmdb"); got["pagerduty"] != nil {
		t.Fatalf("tool metadata leaked cmdb pagerduty field: %+v", got)
	}
	if got := metadataNestedMap(toolView.Metadata, "metrics"); got["raw_payload"] != nil || got["internal_url"] != nil {
		t.Fatalf("tool metadata leaked metrics secrets: %+v", got)
	}
	if got := metadataNestedMap(toolView.Metadata, "runbook"); got["secret"] != nil {
		t.Fatalf("tool metadata leaked runbook secret: %+v", got)
	}
	if got := metadataNestedString(toolView.Metadata, "git", "diff_url"); got == "" {
		t.Fatalf("tool metadata missing allowlisted git field: %+v", toolView.Metadata)
	}
	if got := metadataNestedString(toolView.Metadata, "cmdb", "service_tier"); got == "" {
		t.Fatalf("tool metadata missing allowlisted cmdb field: %+v", toolView.Metadata)
	}
	if got := metadataNestedString(toolView.Metadata, "metrics", "summary"); got == "" {
		t.Fatalf("tool metadata missing allowlisted metrics field: %+v", toolView.Metadata)
	}
	if got := metadataNestedString(toolView.Metadata, "runbook", "url"); got == "" {
		t.Fatalf("tool metadata missing allowlisted runbook field: %+v", toolView.Metadata)
	}
}

func TestAgentRuntime_Analyze_StopsAtMaxRounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"list_evidence","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()

	runtime := NewAgentRuntime(
		AgentConfig{
			BaseURL:              server.URL,
			Model:                "gpt-4.1-mini",
			APIKey:               "local-test-key",
			MaxRounds:            1,
			TimeoutMS:            5000,
			ToolCallingEnabled:   true,
			MaxToolCallsPerRound: 2,
		},
		NewChatCompletionsClient(AgentConfig{
			BaseURL:   server.URL,
			Model:     "gpt-4.1-mini",
			APIKey:    "local-test-key",
			TimeoutMS: 5000,
		}),
	)

	_, err := runtime.Analyze(agentRuntimeTestInput())
	if err == nil {
		t.Fatal("Analyze() error = nil, want max rounds error")
	}
	if !strings.Contains(err.Error(), "max rounds exceeded") {
		t.Fatalf("error = %v, want max rounds exceeded", err)
	}
}

func TestAgentRuntime_Analyze_StopsWhenToolCallsPerRoundExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"list_evidence","arguments":"{}"}},{"id":"call-2","type":"function","function":{"name":"list_rule_signals","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()

	runtime := NewAgentRuntime(
		AgentConfig{
			BaseURL:              server.URL,
			Model:                "gpt-4.1-mini",
			APIKey:               "local-test-key",
			MaxRounds:            2,
			TimeoutMS:            5000,
			ToolCallingEnabled:   true,
			MaxToolCallsPerRound: 1,
		},
		NewChatCompletionsClient(AgentConfig{
			BaseURL:   server.URL,
			Model:     "gpt-4.1-mini",
			APIKey:    "local-test-key",
			TimeoutMS: 5000,
		}),
	)

	_, err := runtime.Analyze(agentRuntimeTestInput())
	if err == nil {
		t.Fatal("Analyze() error = nil, want max tool calls per round error")
	}
	if !strings.Contains(err.Error(), "max tool calls per round exceeded") {
		t.Fatalf("error = %v, want max tool calls per round exceeded", err)
	}
}

func TestAgentRuntime_Analyze_ErrorsWhenToolCallsReturnedWhileDisabled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"list_evidence","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()

	runtime := NewAgentRuntime(
		AgentConfig{
			BaseURL:              server.URL,
			Model:                "gpt-4.1-mini",
			APIKey:               "local-test-key",
			MaxRounds:            2,
			TimeoutMS:            5000,
			ToolCallingEnabled:   false,
			MaxToolCallsPerRound: 2,
		},
		NewChatCompletionsClient(AgentConfig{
			BaseURL:   server.URL,
			Model:     "gpt-4.1-mini",
			APIKey:    "local-test-key",
			TimeoutMS: 5000,
		}),
	)

	_, err := runtime.Analyze(agentRuntimeTestInput())
	if err == nil {
		t.Fatal("Analyze() error = nil, want tool calling disabled error")
	}
	if !strings.Contains(err.Error(), "tool calls returned while tool calling is disabled") {
		t.Fatalf("error = %v, want disabled tool calling error", err)
	}
}

func TestAgentRuntime_Analyze_RejectsMalformedToolCallMetadata(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		wantErr      string
	}{
		{
			name:         "missing id",
			responseBody: `{"choices":[{"message":{"role":"assistant","tool_calls":[{"type":"function","function":{"name":"list_evidence","arguments":"{}"}}]}}]}`,
			wantErr:      "tool call missing id",
		},
		{
			name:         "wrong type",
			responseBody: `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"tool","function":{"name":"list_evidence","arguments":"{}"}}]}}]}`,
			wantErr:      `unsupported tool call type: "tool"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			runtime := NewAgentRuntime(
				AgentConfig{
					BaseURL:              server.URL,
					Model:                "gpt-4.1-mini",
					APIKey:               "local-test-key",
					MaxRounds:            2,
					TimeoutMS:            5000,
					ToolCallingEnabled:   true,
					MaxToolCallsPerRound: 2,
				},
				NewChatCompletionsClient(AgentConfig{
					BaseURL:   server.URL,
					Model:     "gpt-4.1-mini",
					APIKey:    "local-test-key",
					TimeoutMS: 5000,
				}),
			)

			_, err := runtime.Analyze(agentRuntimeTestInput())
			if err == nil {
				t.Fatal("Analyze() error = nil, want malformed tool call error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAgentRuntime_Analyze_RejectsInvalidGetEvidenceByIDArguments(t *testing.T) {
	tests := []struct {
		name      string
		arguments string
		wantErr   string
	}{
		{
			name:      "missing evidence id",
			arguments: `{}`,
			wantErr:   "missing required evidence_id",
		},
		{
			name:      "empty evidence id",
			arguments: `{"evidence_id":"   "}`,
			wantErr:   "missing required evidence_id",
		},
		{
			name:      "unexpected field",
			arguments: `{"evidence_id":"ev-1","extra":"nope"}`,
			wantErr:   "unexpected fields in tool arguments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call-1","type":"function","function":{"name":"get_evidence_by_id","arguments":` + strconv.Quote(tt.arguments) + `}}]}}]}`))
			}))
			defer server.Close()

			runtime := NewAgentRuntime(
				AgentConfig{
					BaseURL:              server.URL,
					Model:                "gpt-4.1-mini",
					APIKey:               "local-test-key",
					MaxRounds:            2,
					TimeoutMS:            5000,
					ToolCallingEnabled:   true,
					MaxToolCallsPerRound: 2,
				},
				NewChatCompletionsClient(AgentConfig{
					BaseURL:   server.URL,
					Model:     "gpt-4.1-mini",
					APIKey:    "local-test-key",
					TimeoutMS: 5000,
				}),
			)

			_, err := runtime.Analyze(agentRuntimeTestInput())
			if err == nil {
				t.Fatal("Analyze() error = nil, want invalid arguments error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseRuntimeHybridAnalysis_NormalizesSuggestedSignalSeverities(t *testing.T) {
	analysis, err := parseRuntimeHybridAnalysis(`{
		"summary":"Runtime completed analysis.",
		"confidence":0.91,
		"requires_human_review":true,
		"rationale":["runtime signal"],
		"suggested_signals":[
			{"signal_name":"normalized_high","severity":" high ","explanation":"normalize casing and whitespace"},
			{"signal_name":"invalid_defaults_low","severity":"SEVERE","explanation":"invalid enum should not pass through"},
			{"signal_name":"missing_defaults_low","explanation":"missing severity should not pass through"}
		]
	}`)
	if err != nil {
		t.Fatalf("parseRuntimeHybridAnalysis() error = %v", err)
	}

	if len(analysis.SuggestedSignals) != 3 {
		t.Fatalf("suggested_signals = %d, want 3", len(analysis.SuggestedSignals))
	}
	if got := analysis.SuggestedSignals[0].Severity; got != RiskHigh {
		t.Fatalf("normalized severity = %q, want %q", got, RiskHigh)
	}
	if got := analysis.SuggestedSignals[1].Severity; got != RiskLow {
		t.Fatalf("invalid severity = %q, want %q fallback", got, RiskLow)
	}
	if got := analysis.SuggestedSignals[2].Severity; got != RiskLow {
		t.Fatalf("missing severity = %q, want %q fallback", got, RiskLow)
	}
}

func agentRuntimeTestInput() AnalyzerInput {
	return AnalyzerInput{
		Bundle: ChangeBundle{
			ChangeID:           "octo/gateway-service#42",
			SourceType:         "pull_request",
			ChangeType:         "pull_request",
			Repo:               "octo/gateway-service",
			Service:            "gateway-service",
			Environment:        "prod",
			Author:             "alice",
			Title:              "adjust auth routing",
			FileList:           []string{"configs/routes.yaml", "internal/auth/handler.go"},
			ImpactedComponents: []string{"gateway", "auth"},
			DiffSummary:        "adjust auth routing and ingress rules",
			ReleaseTarget:      "prod",
		},
		Understanding: ChangeUnderstanding{
			Summary:               "pull_request change targets gateway-service in prod.",
			SemanticTags:          []string{"pull_request", "application_code_change", "auth_logic_changed"},
			RiskHints:             []string{"auth-related component touched", "critical business path detected"},
			ImpactedServices:      []string{"gateway-service"},
			PotentialFailureModes: []string{"traffic misroute or auth regression"},
		},
		Evidence: []EvidenceItem{
			{
				EvidenceID:     "ev-1",
				Type:           "change_bundle",
				Source:         "ingestion",
				Title:          "Normalized change bundle",
				ContentSnippet: "adjust auth routing and ingress rules",
				Confidence:     0.92,
			},
			{
				EvidenceID:     "ev-2",
				Type:           "risk_context",
				Source:         "context_collector",
				Title:          "MVP contextual evidence",
				ContentSnippet: "auth-related component touched; critical business path detected",
				Confidence:     0.86,
			},
		},
	}
}
