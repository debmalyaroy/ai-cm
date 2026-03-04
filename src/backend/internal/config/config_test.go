package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := defaultConfig()

	if cfg.Server.Port != 8080 {
		t.Errorf("default port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeoutSec != 30 {
		t.Errorf("default read timeout = %d, want 30", cfg.Server.ReadTimeoutSec)
	}
	if cfg.Server.WriteTimeoutSec != 60 {
		t.Errorf("default write timeout = %d, want 60", cfg.Server.WriteTimeoutSec)
	}
	if cfg.Server.ShutdownSec != 5 {
		t.Errorf("default shutdown sec = %d, want 5", cfg.Server.ShutdownSec)
	}
	if cfg.Database.MaxConns != 10 {
		t.Errorf("default max conns = %d, want 10", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 2 {
		t.Errorf("default min conns = %d, want 2", cfg.Database.MinConns)
	}
	if cfg.LLM.Provider != "gemini" {
		t.Errorf("default provider = %q, want 'gemini'", cfg.LLM.Provider)
	}
	if cfg.LLM.Temperature != 0.0 {
		t.Errorf("default temperature = %f, want 0.0", cfg.LLM.Temperature)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("default log level = %q, want 'info'", cfg.Logging.Level)
	}
	if cfg.Logging.Format != "json" {
		t.Errorf("default log format = %q, want 'json'", cfg.Logging.Format)
	}
	if len(cfg.CORS.AllowOrigins) != 2 {
		t.Errorf("default CORS origins = %d, want 2", len(cfg.CORS.AllowOrigins))
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg := Load("/nonexistent/config.yaml")
	if cfg == nil {
		t.Fatal("Load should return default config on missing file")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("should fall back to defaults, port = %d", cfg.Server.Port)
	}
}

func TestLoadValidYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
server:
  port: 9090
  read_timeout_sec: 15
database:
  url: "postgres://test:test@localhost/test"
  max_conns: 20
llm:
  provider: "openai"
  openai_model: "gpt-4"
logging:
  level: "debug"
cors:
  allow_origins:
    - "http://example.com"
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load(yamlPath)
	if cfg.Server.Port != 9090 {
		t.Errorf("port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeoutSec != 15 {
		t.Errorf("read timeout = %d, want 15", cfg.Server.ReadTimeoutSec)
	}
	if cfg.Database.URL != "postgres://test:test@localhost/test" {
		t.Errorf("db url mismatch")
	}
	if cfg.Database.MaxConns != 20 {
		t.Errorf("max conns = %d, want 20", cfg.Database.MaxConns)
	}
	if cfg.LLM.Provider != "openai" {
		t.Errorf("provider = %q, want 'openai'", cfg.LLM.Provider)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("log level = %q, want 'debug'", cfg.Logging.Level)
	}
	if len(cfg.CORS.AllowOrigins) != 1 || cfg.CORS.AllowOrigins[0] != "http://example.com" {
		t.Errorf("cors origins = %v", cfg.CORS.AllowOrigins)
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(yamlPath, []byte(":::invalid:::yaml:::"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := Load(yamlPath)
	if cfg == nil {
		t.Fatal("should return defaults on invalid YAML")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("should fall back to defaults on parse error")
	}
}

func TestEnvOverrides(t *testing.T) {
	// Clean up after
	orig := map[string]string{}
	envs := []string{"PORT", "DATABASE_URL", "LLM_PROVIDER", "GEMINI_API_KEY", "GEMINI_MODEL", "OPENAI_API_KEY", "OPENAI_MODEL", "LOG_LEVEL"}
	for _, e := range envs {
		orig[e] = os.Getenv(e)
	}
	defer func() {
		for k, v := range orig {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	os.Setenv("PORT", "7777")
	os.Setenv("DATABASE_URL", "postgres://env:env@envhost/envdb")
	os.Setenv("LLM_PROVIDER", "openai")
	os.Setenv("GEMINI_API_KEY", "gkey123")
	os.Setenv("GEMINI_MODEL", "gemini-pro")
	os.Setenv("OPENAI_API_KEY", "okey456")
	os.Setenv("OPENAI_MODEL", "gpt-4-turbo")
	os.Setenv("LOG_LEVEL", "error")

	cfg := Load("/nonexistent.yaml")

	if cfg.Server.Port != 7777 {
		t.Errorf("port = %d, want 7777", cfg.Server.Port)
	}
	if cfg.Database.URL != "postgres://env:env@envhost/envdb" {
		t.Errorf("db url = %q", cfg.Database.URL)
	}
	if cfg.LLM.Provider != "openai" {
		t.Errorf("provider = %q, want 'openai'", cfg.LLM.Provider)
	}
	if cfg.LLM.GeminiKey != "gkey123" {
		t.Errorf("gemini key = %q", cfg.LLM.GeminiKey)
	}
	if cfg.LLM.GeminiModel != "gemini-pro" {
		t.Errorf("gemini model = %q", cfg.LLM.GeminiModel)
	}
	if cfg.LLM.OpenAIKey != "okey456" {
		t.Errorf("openai key = %q", cfg.LLM.OpenAIKey)
	}
	if cfg.LLM.OpenAIModel != "gpt-4-turbo" {
		t.Errorf("openai model = %q", cfg.LLM.OpenAIModel)
	}
	if cfg.Logging.Level != "error" {
		t.Errorf("log level = %q, want 'error'", cfg.Logging.Level)
	}
}

func TestEnvOverridesYAML(t *testing.T) {
	orig := os.Getenv("PORT")
	defer func() {
		if orig == "" {
			os.Unsetenv("PORT")
		} else {
			os.Setenv("PORT", orig)
		}
	}()

	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
server:
  port: 9090
`
	os.WriteFile(yamlPath, []byte(yamlContent), 0644)

	// Env should override YAML
	os.Setenv("PORT", "5555")
	cfg := Load(yamlPath)
	if cfg.Server.Port != 5555 {
		t.Errorf("env should override YAML: port = %d, want 5555", cfg.Server.Port)
	}
}

func TestInvalidPortEnv(t *testing.T) {
	orig := os.Getenv("PORT")
	defer func() {
		if orig == "" {
			os.Unsetenv("PORT")
		} else {
			os.Setenv("PORT", orig)
		}
	}()

	os.Setenv("PORT", "not-a-number")
	cfg := Load("/none.yaml")
	if cfg.Server.Port != 8080 {
		t.Errorf("invalid PORT var should not change default: port = %d", cfg.Server.Port)
	}
}
