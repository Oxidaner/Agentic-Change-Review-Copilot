package review

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgentConfig_MergesPublicAndLocalAndDefaults(t *testing.T) {
	publicPath := writeAgentConfigFixture(t, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini",
  "max_rounds": 0,
  "timeout_ms": 0,
  "tool_calling_enabled": true,
  "max_tool_calls_per_round": 0
}`)
	localPath := writeAgentConfigFixture(t, `{
  "api_key": "sk-local-123",
  "max_rounds": 5
}`)

	cfg, err := LoadAgentConfig(publicPath, localPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}
	if cfg.Model != "gpt-4.1-mini" {
		t.Fatalf("Model = %q, want public model", cfg.Model)
	}
	if cfg.APIKey != "sk-local-123" {
		t.Fatalf("APIKey = %q, want merged local api key", cfg.APIKey)
	}
	if cfg.MaxRounds != 5 {
		t.Fatalf("MaxRounds = %d, want local override", cfg.MaxRounds)
	}
	if cfg.TimeoutMS != 15000 {
		t.Fatalf("TimeoutMS = %d, want default 15000", cfg.TimeoutMS)
	}
	if cfg.MaxToolCallsPerRound != 4 {
		t.Fatalf("MaxToolCallsPerRound = %d, want default 4", cfg.MaxToolCallsPerRound)
	}
}

func TestLoadAgentConfig_LocalBooleanFalseOverridesPublicTrue(t *testing.T) {
	publicPath := writeAgentConfigFixture(t, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini",
  "tool_calling_enabled": true
}`)
	localPath := writeAgentConfigFixture(t, `{
  "enabled": false,
  "tool_calling_enabled": false
}`)

	cfg, err := LoadAgentConfig(publicPath, localPath)
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}
	if cfg.Enabled {
		t.Fatal("expected local enabled=false to override public enabled=true")
	}
	if cfg.ToolCallingEnabled {
		t.Fatal("expected local tool_calling_enabled=false to override public true")
	}
}

func TestLoadAgentConfig_MissingLocalStillWorks(t *testing.T) {
	publicPath := writeAgentConfigFixture(t, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4.1-mini",
  "api_key": "sk-public-123"
}`)

	cfg, err := LoadAgentConfig(publicPath, filepath.Join(t.TempDir(), "does-not-exist.local.json"))
	if err != nil {
		t.Fatalf("LoadAgentConfig() error = %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected config to remain enabled from public config")
	}
	if cfg.APIKey != "sk-public-123" {
		t.Fatalf("APIKey = %q, want public api key", cfg.APIKey)
	}
}

func TestLoadAgentConfig_ValidatesMergedConfig(t *testing.T) {
	publicPath := writeAgentConfigFixture(t, `{
  "enabled": true,
  "base_url": "https://api.openai.com/v1"
}`)
	localPath := writeAgentConfigFixture(t, `{
  "api_key": "sk-local-123"
}`)

	_, err := LoadAgentConfig(publicPath, localPath)
	if err == nil {
		t.Fatal("expected merged config validation error")
	}
}

func writeAgentConfigFixture(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "review-agent.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}
