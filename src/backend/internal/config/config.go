package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all application configuration
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	LLM      LLMConfig      `yaml:"llm"`
	Logging  LoggingConfig  `yaml:"logging"`
	CORS     CORSConfig     `yaml:"cors"`
	Security SecurityConfig `yaml:"security"`
}

type ServerConfig struct {
	Port            int `yaml:"port"`
	ReadTimeoutSec  int `yaml:"read_timeout_sec"`
	WriteTimeoutSec int `yaml:"write_timeout_sec"`
	ShutdownSec     int `yaml:"shutdown_sec"`
}

type DatabaseConfig struct {
	URL            string `yaml:"url"`
	MaxConns       int    `yaml:"max_conns"`
	MinConns       int    `yaml:"min_conns"`
	MaxConnLifeSec int    `yaml:"max_conn_life_sec"`
}

type LLMConfig struct {
	Provider    string            `yaml:"provider"`
	GeminiKey   string            `yaml:"gemini_api_key"`
	GeminiModel string            `yaml:"gemini_model"`
	OpenAIKey   string            `yaml:"openai_api_key"`
	OpenAIModel string            `yaml:"openai_model"`
	AWSRegion   string            `yaml:"aws_region"`
	AWSModel    string            `yaml:"aws_model"`
	LocalURL    string            `yaml:"local_url"`
	LocalModel  string            `yaml:"local_model"`
	Temperature float32           `yaml:"temperature"`
	AgentModels map[string]string `yaml:"agent_models"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type CORSConfig struct {
	AllowOrigins []string `yaml:"allow_origins"`
}

type SecurityConfig struct {
	RateLimitEnabled   bool     `yaml:"rate_limit_enabled"`
	RateLimitPerMinute int      `yaml:"rate_limit_per_minute"`
	APIKeyAuthEnabled  bool     `yaml:"api_key_auth_enabled"`
	APIKeys            []string `yaml:"api_keys"`
}

// Load reads config from YAML file, then overrides with env vars.
// Env vars take precedence (for secrets via .env).
func Load(path string) *Config {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Info("config: no YAML file found, using defaults + env", "path", path)
	} else {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Warn("config: failed to parse YAML file, using defaults", "path", path, "error", err)
		}
	}

	// Env overrides (secrets + runtime)
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	} else {
		// Expand any environment variables in the URL from the YAML file (e.g. ${POSTGRES_USER})
		cfg.Database.URL = os.ExpandEnv(cfg.Database.URL)
	}

	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = p
		}
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := os.Getenv("GEMINI_API_KEY"); v != "" {
		cfg.LLM.GeminiKey = v
	}
	if v := os.Getenv("GEMINI_MODEL"); v != "" {
		cfg.LLM.GeminiModel = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		cfg.LLM.OpenAIKey = v
	}
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		cfg.LLM.OpenAIModel = v
	}
	if v := os.Getenv("AWS_REGION"); v != "" {
		cfg.LLM.AWSRegion = v
	}
	if v := os.Getenv("AWS_BEDROCK_MODEL"); v != "" {
		cfg.LLM.AWSModel = v
	}
	if v := os.Getenv("LOCAL_LLM_URL"); v != "" {
		cfg.LLM.LocalURL = v
	}
	if v := os.Getenv("LOCAL_LLM_MODEL"); v != "" {
		cfg.LLM.LocalModel = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Logging.Level = v
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.CORS.AllowOrigins = strings.Split(v, ",")
	}

	return cfg
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Port:            8080,
			ReadTimeoutSec:  30,
			WriteTimeoutSec: 60,
			ShutdownSec:     5,
		},
		Database: DatabaseConfig{
			URL:            "postgres://aicm:aicm_secret@localhost:5432/aicm?sslmode=disable",
			MaxConns:       10,
			MinConns:       2,
			MaxConnLifeSec: 3600,
		},
		LLM: LLMConfig{
			Provider:    "gemini",
			GeminiModel: "gemini-2.0-flash",
			OpenAIModel: "gpt-4o-mini",
			AWSRegion:   "us-east-1",
			AWSModel:    "anthropic.claude-3-haiku-20240307-v1:0",
			LocalURL:    "http://localhost:11434/api/generate",
			LocalModel:  "llama3",
			Temperature: 0.0,
			AgentModels: map[string]string{},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		CORS: CORSConfig{
			AllowOrigins: []string{"http://localhost:3000", "http://localhost:3001"},
		},
		Security: SecurityConfig{
			RateLimitEnabled:   true,
			RateLimitPerMinute: 30,
			APIKeyAuthEnabled:  false,
			APIKeys:            []string{},
		},
	}
}
