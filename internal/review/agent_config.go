package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// AgentConfig controls the optional LLM-backed review agent.
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

// LoadAgentConfig loads the public config and merges an optional local override.
func LoadAgentConfig(publicPath, localPath string) (AgentConfig, error) {
	publicDoc, err := loadAgentConfigFile(publicPath)
	if err != nil {
		return AgentConfig{}, err
	}

	cfg := publicDoc.toConfig()
	if localPath != "" {
		localDoc, err := loadAgentConfigFile(localPath)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return AgentConfig{}, err
			}
		} else {
			localDoc.applyTo(&cfg)
		}
	}

	return validateAgentConfig(cfg)
}

func loadAgentConfigFile(path string) (agentConfigDocument, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return agentConfigDocument{}, err
	}

	var doc agentConfigDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return agentConfigDocument{}, fmt.Errorf("decode agent config %s: %w", path, err)
	}
	return doc, nil
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

type agentConfigDocument struct {
	Enabled              *bool  `json:"enabled"`
	BaseURL              string `json:"base_url"`
	Model                string `json:"model"`
	APIKey               string `json:"api_key"`
	MaxRounds            int    `json:"max_rounds"`
	TimeoutMS            int    `json:"timeout_ms"`
	ToolCallingEnabled   *bool  `json:"tool_calling_enabled"`
	MaxToolCallsPerRound int    `json:"max_tool_calls_per_round"`
}

func (doc agentConfigDocument) toConfig() AgentConfig {
	cfg := AgentConfig{
		BaseURL:              doc.BaseURL,
		Model:                doc.Model,
		APIKey:               doc.APIKey,
		MaxRounds:            doc.MaxRounds,
		TimeoutMS:            doc.TimeoutMS,
		MaxToolCallsPerRound: doc.MaxToolCallsPerRound,
	}
	if doc.Enabled != nil {
		cfg.Enabled = *doc.Enabled
	}
	if doc.ToolCallingEnabled != nil {
		cfg.ToolCallingEnabled = *doc.ToolCallingEnabled
	}
	return cfg
}

func (doc agentConfigDocument) applyTo(cfg *AgentConfig) {
	if doc.Enabled != nil {
		cfg.Enabled = *doc.Enabled
	}
	if doc.BaseURL != "" {
		cfg.BaseURL = doc.BaseURL
	}
	if doc.Model != "" {
		cfg.Model = doc.Model
	}
	if doc.APIKey != "" {
		cfg.APIKey = doc.APIKey
	}
	if doc.MaxRounds != 0 {
		cfg.MaxRounds = doc.MaxRounds
	}
	if doc.TimeoutMS != 0 {
		cfg.TimeoutMS = doc.TimeoutMS
	}
	if doc.ToolCallingEnabled != nil {
		cfg.ToolCallingEnabled = *doc.ToolCallingEnabled
	}
	if doc.MaxToolCallsPerRound != 0 {
		cfg.MaxToolCallsPerRound = doc.MaxToolCallsPerRound
	}
}
