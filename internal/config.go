package buzzhive

import (
	"os"

	"gopkg.in/yaml.v3"
)

func loadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = "127.0.0.1:9622"
	}
	if cfg.Upstream.BaseURL == "" {
		cfg.Upstream.BaseURL = "https://generativelanguage.googleapis.com"
	}
	if cfg.Upstream.Timeout == "" {
		cfg.Upstream.Timeout = "10m"
	}
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "sqlite"
	}
	if cfg.Database.Driver == "sqlite" && cfg.Database.Path == "" {
		cfg.Database.Path = "data/buzzhive.db"
	}
	if envURL := os.Getenv("BUZZHIVE_DATABASE_URL"); envURL != "" {
		cfg.Database.Driver = "postgres"
		cfg.Database.URL = envURL
	}
	if cfg.Retry.MaxAttempts <= 0 {
		cfg.Retry.MaxAttempts = 8
	}
	if cfg.Retry.CooldownSeconds <= 0 {
		cfg.Retry.CooldownSeconds = 60
	}
	if len(cfg.Models.Auto) == 0 {
		cfg.Models.Auto = []string{"gemini-3.5-flash", "gemini-3-flash-preview", "gemini-3.1-flash-lite"}
	}
	return cfg, nil
}
