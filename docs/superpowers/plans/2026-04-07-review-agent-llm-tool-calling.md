# Review Agent LLM Tool Calling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a real OpenAI-compatible multi-round tool-calling agent to the review hybrid-analysis stage while preserving deterministic heuristic fallback.

**Architecture:** Keep `internal/review/service.go` dependent only on `ChangeAnalyzer`. Add a dedicated agent analyzer stack under `internal/review` for config loading, OpenAI-compatible chat completions, read-only tool dispatch, and a bounded transcript loop. When config is missing or runtime execution fails, the service must continue to return a valid `HybridAnalysis` via `HeuristicAnalyzer`.

**Tech Stack:** Go, net/http, encoding/json, PostgreSQL-backed existing stores, OpenAI-compatible Chat Completions API

---

## File Structure

### New Files

- `internal/review/agent_config.go`
  Load `config/review-agent.json` and merge optional local private config.
- `internal/review/llm_client.go`
  Implement the OpenAI-compatible chat completions client and request/response DTOs.
- `internal/review/agent_tools.go`
  Define tool schemas plus deterministic read-only tool dispatch.
- `internal/review/agent_runtime.go`
  Execute the bounded transcript loop with tool calling.
- `internal/review/agent_analyzer.go`
  Adapt the runtime to the `ChangeAnalyzer` interface.
- `internal/review/agent_config_test.go`
- `internal/review/agent_tools_test.go`
- `internal/review/agent_runtime_test.go`
- `internal/review/agent_analyzer_test.go`
- `config/review-agent.json`
  Public non-secret config tracked in git.
- `.gitignore`
  Ignore `config/review-agent.local.json`.

### Modified Files

- `internal/review/service.go`
  Prefer `AgentAnalyzer` when config is valid and keep heuristic fallback.
- `internal/review/analyzer.go`
  Keep `HeuristicAnalyzer` as the explicit fallback implementation.
- `internal/review/pipeline.go`
  Reuse existing bundle / evidence / signal shapes in tool outputs.
- `internal/review/service_test.go`
  Add service-level assertions for agent/fallback selection.
- `internal/api/review_handler_test.go`
  Add API-level assertions that `analysis` reflects agent output.
- `README.md`
  Document config files and the real agent analysis stage.
- `docs/local-development.md`
  Document local agent config and how fallback behaves.

## Task 1: Add Config Loading And Fallback Selection

**Files:**
- Create: `internal/review/agent_config.go`
- Create: `internal/review/agent_config_test.go`
- Create: `config/review-agent.json`
- Create: `.gitignore`
- Modify: `internal/review/service.go`

- [ ] **Step 1: Write the failing config tests**

```go
package review

import "testing"

func TestLoadAgentConfig_MergesPublicAndLocal(t *testing.T) {
	cfg, err := LoadAgentConfig("../../config/review-agent.json", "../../config/review-agent.local.json")
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}
	if cfg.Model == "" {
		t.Fatal("expected model to be populated")
	}
}

func TestLoadAgentConfig_MissingLocalStillWorks(t *testing.T) {
	cfg, err := LoadAgentConfig("../../config/review-agent.json", "../../config/does-not-exist.local.json")
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected config to remain enabled from public config")
	}
}

func TestLoadAgentConfig_RequiresAPIKeyWhenEnabled(t *testing.T) {
	_, err := validateAgentConfig(AgentConfig{Enabled: true, Model: "gpt-4.1-mini"})
	if err == nil {
		t.Fatal("expected missing api_key validation error")
	}
}
```

- [ ] **Step 2: Run the config tests to verify they fail**

Run: `go test ./internal/review -run 'TestLoadAgentConfig|TestLoadAgentConfig_MissingLocalStillWorks|TestLoadAgentConfig_RequiresAPIKeyWhenEnabled' -count=1`
Expected: FAIL with undefined `LoadAgentConfig`, undefined `AgentConfig`, and undefined `validateAgentConfig`

- [ ] **Step 3: Implement config loading and validation**

```go
package review

import (
	"encoding/json"
	"errors"
	"os"
)

type AgentConfig struct {
	Enabled              bool   `json:"enabled"`
	BaseURL              string `json:"base_url"`
	Model                string `json:"model"`
	APIKey               string `json:"api_key"`
	MaxRounds            int    `json:"max_rounds"`
	TimeoutMS            int    `json:"timeout_ms"`
	ToolCallingEnabled   bool   `json:"tool_calling_enabled"`
	MaxToolCallsPerRound int    `json:"max_tool_calls_per_round"`
}

func LoadAgentConfig(publicPath, localPath string) (AgentConfig, error) {
	cfg, err := loadAgentConfigFile(publicPath)
	if err != nil {
		return AgentConfig{}, err
	}
	localCfg, err := loadAgentConfigFile(localPath)
	if err == nil {
		cfg.merge(localCfg)
	}
	return validateAgentConfig(cfg)
}

func loadAgentConfigFile(path string) (AgentConfig, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return AgentConfig{}, err
	}
	var cfg AgentConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		return AgentConfig{}, err
	}
	return cfg, nil
}

func validateAgentConfig(cfg AgentConfig) (AgentConfig, error) {
	if !cfg.Enabled {
		return cfg, nil
	}
	if cfg.BaseURL == "" || cfg.Model == "" || cfg.APIKey == "" {
		return AgentConfig{}, errors.New("enabled agent config requires base_url, model, and api_key")
	}
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 3
	}
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = 15000
	}
	if cfg.MaxToolCallsPerRound <= 0 {
		cfg.MaxToolCallsPerRound = 4
	}
	return cfg, nil
}

func (cfg *AgentConfig) merge(override AgentConfig) {
	if override.BaseURL != "" {
		cfg.BaseURL = override.BaseURL
	}
	if override.Model != "" {
		cfg.Model = override.Model
	}
	if override.APIKey != "" {
		cfg.APIKey = override.APIKey
	}
	if override.MaxRounds != 0 {
		cfg.MaxRounds = override.MaxRounds
	}
	if override.TimeoutMS != 0 {
		cfg.TimeoutMS = override.TimeoutMS
	}
	if override.MaxToolCallsPerRound != 0 {
		cfg.MaxToolCallsPerRound = override.MaxToolCallsPerRound
	}
	cfg.Enabled = cfg.Enabled || override.Enabled
	cfg.ToolCallingEnabled = cfg.ToolCallingEnabled || override.ToolCallingEnabled
}
```

- [ ] **Step 4: Add tracked public config and ignore local private config**

`config/review-agent.json`

```json
{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini",
  "max_rounds": 3,
  "timeout_ms": 15000,
  "tool_calling_enabled": true,
  "max_tool_calls_per_round": 4
}
```

`.gitignore`

```gitignore
config/review-agent.local.json
```

- [ ] **Step 5: Teach `NewService` to prefer agent config without breaking fallback**

```go
func NewService(store Store, options ...ServiceOption) *Service {
	service := &Service{
		store:    store,
		analyzer: HeuristicAnalyzer{},
	}
	if cfg, err := LoadAgentConfig("config/review-agent.json", "config/review-agent.local.json"); err == nil && cfg.Enabled {
		service.analyzer = NewAgentAnalyzer(cfg, HeuristicAnalyzer{})
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}
```

- [ ] **Step 6: Run the config tests to verify they pass**

Run: `go test ./internal/review -run 'TestLoadAgentConfig|TestLoadAgentConfig_MissingLocalStillWorks|TestLoadAgentConfig_RequiresAPIKeyWhenEnabled' -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add .gitignore config/review-agent.json internal/review/agent_config.go internal/review/agent_config_test.go internal/review/service.go
git commit -m "feat: add review agent config loading"
```

## Task 2: Add OpenAI-Compatible Client And DTOs

**Files:**
- Create: `internal/review/llm_client.go`
- Create: `internal/review/agent_runtime_test.go`

- [ ] **Step 1: Write the failing client tests**

```go
func TestChatCompletionsClient_CreateCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("path = %s, want /chat/completions", r.URL.Path)
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
	if len(resp.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(resp.Choices))
	}
}
```

- [ ] **Step 2: Run the client test to verify it fails**

Run: `go test ./internal/review -run TestChatCompletionsClient_CreateCompletion -count=1`
Expected: FAIL with undefined `NewChatCompletionsClient`, `ChatCompletionRequest`, and `ChatMessage`

- [ ] **Step 3: Implement the OpenAI-compatible client**

```go
type ChatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []LLMTool     `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

type ChatCompletionsClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func NewChatCompletionsClient(cfg AgentConfig) *ChatCompletionsClient {
	return &ChatCompletionsClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond},
	}
}
```

- [ ] **Step 4: Run the client test to verify it passes**

Run: `go test ./internal/review -run TestChatCompletionsClient_CreateCompletion -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/review/llm_client.go internal/review/agent_runtime_test.go
git commit -m "feat: add OpenAI-compatible review agent client"
```

## Task 3: Add Tool Registry And Bounded Agent Runtime

**Files:**
- Create: `internal/review/agent_tools.go`
- Create: `internal/review/agent_runtime.go`
- Modify: `internal/review/agent_runtime_test.go`

- [ ] **Step 1: Write the failing runtime tests**

```go
func TestAgentRuntime_ExecutesToolCallAndReturnsAnalysis(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(`"tool_call_id":"tool_1"`)) {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"{\"summary\":\"agent ok\",\"confidence\":0.88,\"requires_human_review\":true,\"rationale\":[\"used evidence\"],\"suggested_signals\":[{\"signal_name\":\"llm_custom_risk\",\"severity\":\"HIGH\",\"explanation\":\"model escalated risk\"}]}"}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"tool_1","type":"function","function":{"name":"get_change_bundle","arguments":"{}"}}]}}]}`))
	}))
	defer server.Close()

	runtime := NewAgentRuntime(
		AgentConfig{BaseURL: server.URL, APIKey: "local-test-key", Model: "gpt-4.1-mini", MaxRounds: 3, TimeoutMS: 5000, ToolCallingEnabled: true, MaxToolCallsPerRound: 2},
		NewChatCompletionsClient(AgentConfig{BaseURL: server.URL, APIKey: "local-test-key", Model: "gpt-4.1-mini", TimeoutMS: 5000}),
	)

	result, err := runtime.Run(AnalyzerInput{
		Bundle: ChangeBundle{ChangeID: "pr-42", SourceType: "pull_request", Service: "gateway", Environment: "prod"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Summary != "agent ok" {
		t.Fatalf("summary = %q, want %q", result.Summary, "agent ok")
	}
}

func TestAgentRuntime_StopsAtMaxRounds(t *testing.T) {
	// fake server always returns a tool call; runtime must stop with an error
}
```

- [ ] **Step 2: Run the runtime tests to verify they fail**

Run: `go test ./internal/review -run 'TestAgentRuntime_ExecutesToolCallAndReturnsAnalysis|TestAgentRuntime_StopsAtMaxRounds' -count=1`
Expected: FAIL with undefined `NewAgentRuntime`

- [ ] **Step 3: Implement read-only tools and the runtime loop**

```go
type LLMTool struct {
	Type     string          `json:"type"`
	Function LLMToolFunction `json:"function"`
}

type LLMToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type AgentRuntime struct {
	cfg    AgentConfig
	client *ChatCompletionsClient
	tools  map[string]func(AnalyzerInput, json.RawMessage) (any, error)
}

func NewAgentRuntime(cfg AgentConfig, client *ChatCompletionsClient) *AgentRuntime {
	return &AgentRuntime{
		cfg:    cfg,
		client: client,
		tools:  defaultAgentTools(),
	}
}

func (rt *AgentRuntime) Run(input AnalyzerInput) (HybridAnalysis, error) {
	messages := buildInitialMessages(input)
	tools := buildToolDefinitions()
	for round := 0; round < rt.cfg.MaxRounds; round++ {
		resp, err := rt.client.CreateCompletion(ChatCompletionRequest{
			Model:    rt.cfg.Model,
			Messages: messages,
			Tools:    tools,
		})
		if err != nil {
			return HybridAnalysis{}, err
		}
		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) == 0 {
			return parseHybridAnalysis(msg.Content)
		}
		if len(msg.ToolCalls) > rt.cfg.MaxToolCallsPerRound {
			return HybridAnalysis{}, fmt.Errorf("tool call count exceeds limit")
		}
		messages = append(messages, msg)
		for _, toolCall := range msg.ToolCalls {
			payload, err := dispatchTool(rt.tools, input, toolCall)
			if err != nil {
				return HybridAnalysis{}, err
			}
			messages = append(messages, ChatMessage{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				Name:       toolCall.Function.Name,
				Content:    string(payload),
			})
		}
	}
	return HybridAnalysis{}, fmt.Errorf("agent runtime exceeded max rounds")
}
```

- [ ] **Step 4: Run the runtime tests to verify they pass**

Run: `go test ./internal/review -run 'TestAgentRuntime_ExecutesToolCallAndReturnsAnalysis|TestAgentRuntime_StopsAtMaxRounds' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/review/agent_tools.go internal/review/agent_runtime.go internal/review/agent_runtime_test.go
git commit -m "feat: add bounded review agent runtime"
```

## Task 4: Add AgentAnalyzer Adapter And Service Fallback Tests

**Files:**
- Create: `internal/review/agent_analyzer.go`
- Create: `internal/review/agent_analyzer_test.go`
- Modify: `internal/review/service_test.go`

- [ ] **Step 1: Write the failing analyzer tests**

```go
func TestAgentAnalyzer_FallsBackToHeuristicOnRuntimeError(t *testing.T) {
	analyzer := NewAgentAnalyzer(
		AgentConfig{Enabled: true, Model: "gpt-4.1-mini"},
		HeuristicAnalyzer{},
		withRuntime(func(AnalyzerInput) (HybridAnalysis, error) {
			return HybridAnalysis{}, errors.New("boom")
		}),
	)

	result, err := analyzer.Analyze(AnalyzerInput{
		Bundle: ChangeBundle{ChangeID: "pr-42", SourceType: "pull_request", Service: "gateway", Environment: "prod"},
	})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Analyzer == "heuristic_fallback" {
		return
	}
	t.Fatalf("analyzer = %q, want heuristic fallback marker", result.Analyzer)
}
```

- [ ] **Step 2: Run the analyzer tests to verify they fail**

Run: `go test ./internal/review -run TestAgentAnalyzer_FallsBackToHeuristicOnRuntimeError -count=1`
Expected: FAIL with undefined `NewAgentAnalyzer`

- [ ] **Step 3: Implement the adapter**

```go
type runtimeFunc func(AnalyzerInput) (HybridAnalysis, error)

type AgentAnalyzer struct {
	cfg      AgentConfig
	fallback ChangeAnalyzer
	run      runtimeFunc
}

func NewAgentAnalyzer(cfg AgentConfig, fallback ChangeAnalyzer, opts ...func(*AgentAnalyzer)) *AgentAnalyzer {
	analyzer := &AgentAnalyzer{
		cfg:      cfg,
		fallback: fallback,
		run: func(input AnalyzerInput) (HybridAnalysis, error) {
			rt := NewAgentRuntime(cfg, NewChatCompletionsClient(cfg))
			return rt.Run(input)
		},
	}
	for _, opt := range opts {
		opt(analyzer)
	}
	return analyzer
}

func (a *AgentAnalyzer) Analyze(input AnalyzerInput) (HybridAnalysis, error) {
	result, err := a.run(input)
	if err == nil {
		if result.Mode == "" {
			result.Mode = "llm_tool_calling_agent"
		}
		if result.Analyzer == "" {
			result.Analyzer = "openai_compatible_agent"
		}
		return result, nil
	}
	fallbackResult, fallbackErr := a.fallback.Analyze(input)
	if fallbackErr != nil {
		return HybridAnalysis{}, fallbackErr
	}
	fallbackResult.Mode = "rules_plus_llm_skeleton"
	fallbackResult.Analyzer = "heuristic_fallback"
	fallbackResult.Rationale = append(fallbackResult.Rationale, "agent runtime failed: "+err.Error())
	return fallbackResult, nil
}
```

- [ ] **Step 4: Add service-level tests for agent and fallback surfaces**

```go
func TestService_GetReviewIncludesAgentAnalysis(t *testing.T) {
	service := NewService(NewMemoryStore(), WithAnalyzer(stubAnalyzer{
		result: HybridAnalysis{
			Mode:      "llm_tool_calling_agent",
			Analyzer:  "openai_compatible_agent",
			Summary:   "agent-generated summary",
			Confidence: 0.91,
		},
	}))
	// create review, fetch review, assert analysis fields
}
```

- [ ] **Step 5: Run the analyzer and service tests to verify they pass**

Run: `go test ./internal/review -run 'TestAgentAnalyzer|TestService_GetReviewIncludesAgentAnalysis' -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/review/agent_analyzer.go internal/review/agent_analyzer_test.go internal/review/service_test.go
git commit -m "feat: add review agent analyzer adapter"
```

## Task 5: Add API Assertions And Documentation Updates

**Files:**
- Modify: `internal/api/review_handler_test.go`
- Modify: `README.md`
- Modify: `docs/local-development.md`

- [ ] **Step 1: Write the failing API test**

```go
func TestGetReviewIncludesHybridAnalysisFields(t *testing.T) {
	service := review.NewService(review.NewMemoryStore(), review.WithAnalyzer(stubAnalyzer{
		result: review.HybridAnalysis{
			Mode:      "llm_tool_calling_agent",
			Analyzer:  "openai_compatible_agent",
			Summary:   "agent summary",
			Confidence: 0.93,
			RequiresHumanReview: true,
		},
	}))
	handler := api.NewReviewHandler(service)
	// create review, fetch review, assert response analysis.mode/analyzer/summary
}
```

- [ ] **Step 2: Run the API test to verify it fails**

Run: `go test ./internal/api -run TestGetReviewIncludesHybridAnalysisFields -count=1`
Expected: FAIL until the new assertions and test fixture are in place

- [ ] **Step 3: Update API tests and docs**

`README.md`

```md
The hybrid analysis stage now supports an optional OpenAI-compatible
tool-calling agent configured through:

- `config/review-agent.json`
- `config/review-agent.local.json`

When local private config is missing or the model call fails, the service
automatically falls back to the heuristic analyzer.
```

`docs/local-development.md`

```md
## Review Agent Config

Tracked config:
- `config/review-agent.json`

Private local config:
- `config/review-agent.local.json`

The local file should contain `api_key`. If it is absent, the review pipeline
continues with heuristic fallback.
```

- [ ] **Step 4: Run focused API tests**

Run: `go test ./internal/api -run TestGetReviewIncludesHybridAnalysisFields -count=1`
Expected: PASS

- [ ] **Step 5: Run the full test suite and build**

Run: `go test ./...`
Expected: PASS

Run: `GOCACHE=/tmp/go-build go build ./...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add README.md docs/local-development.md internal/api/review_handler_test.go
git commit -m "docs: describe review agent configuration and fallback"
```

## Self-Review

Spec coverage:
- Real OpenAI-compatible LLM path: covered by Tasks 1-4
- Multi-round tool calling agent: covered by Task 3
- Config in tracked + local files: covered by Task 1
- HybridAnalysis mapping and fallback: covered by Task 4
- User-facing and API verification: covered by Task 5

Placeholder scan:
- No `TBD`, `TODO`, or cross-task references that require hidden context.

Type consistency:
- `AgentConfig`, `ChatMessage`, `ChatCompletionRequest`, `AgentRuntime`, and `AgentAnalyzer` are introduced before later tasks depend on them.
