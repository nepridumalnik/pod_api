package config

import (
	"fmt"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

type Config struct {
	// HTTP listen address, e.g. ":8080"
	Address string `env:"ADDRESS" envDefault:":8080"`
}

// Load loads .env (if present) and parses environment variables into Config.
func Load() (Config, error) {
	// Load .env if available; ignore error if file does not exist
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
