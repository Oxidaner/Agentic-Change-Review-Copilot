package review

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ChatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
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

type LLMTool struct {
	Type     string          `json:"type"`
	Function LLMToolFunction `json:"function"`
}

type LLMToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type ChatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Tools    []LLMTool     `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	Choices []ChatCompletionChoice `json:"choices"`
}

type ChatCompletionChoice struct {
	Message ChatMessage `json:"message"`
}

type ChatCompletionsClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
	model   string
}

func NewChatCompletionsClient(cfg AgentConfig) *ChatCompletionsClient {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &ChatCompletionsClient{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		client:  &http.Client{Timeout: timeout},
		model:   cfg.Model,
	}
}

func (c *ChatCompletionsClient) CreateCompletion(req ChatCompletionRequest) (ChatCompletionResponse, error) {
	if c == nil {
		return ChatCompletionResponse{}, fmt.Errorf("chat completions client is nil")
	}

	if req.Model == "" {
		req.Model = c.model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return ChatCompletionResponse{}, err
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return ChatCompletionResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return ChatCompletionResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		bodyText := strings.TrimSpace(string(body))
		if bodyText != "" {
			return ChatCompletionResponse{}, fmt.Errorf("chat completions request failed: %s: %s", resp.Status, bodyText)
		}
		return ChatCompletionResponse{}, fmt.Errorf("chat completions request failed: %s", resp.Status)
	}

	var out ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ChatCompletionResponse{}, err
	}
	return out, nil
}
