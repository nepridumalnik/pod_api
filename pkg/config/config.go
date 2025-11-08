package config

import (
	"fmt"

	"github.com/caarlos0/env/v9"
	"github.com/joho/godotenv"
)

type Config struct {
	Server struct {
		Port uint16 `env:"PORT" envDefault:"8080"`

		Host string `env:"HOST" envDefault:"0.0.0.0"`
	}

	OpenAI struct {
		URL string `env:"OPENAI_URL" envDefault:"https://api.aitunnel.ru/v1"`

		BasicKey string `env:"OPENAI_BASIC_KEY,required"`
	}

	Gigachat struct {
		// Base API URL for GigaChat, should point to API root (ends without trailing slash)
		URL string `env:"GIGACHAT_URL" envDefault:"https://gigachat.devices.sberbank.ru/api/v1"`

		// Auth API base for OAuth token requests (without trailing slash)
		AuthURL string `env:"GIGACHAT_AUTH_URL" envDefault:"https://ngw.devices.sberbank.ru:9443/api/v2"`

		// Default model for chat completions
		// Allowed: GigaChat-2, GigaChat-2-Pro, GigaChat-2-Max
		Model string `env:"GIGACHAT_MODEL" envDefault:"GigaChat-2"`

		// OAuth scope to request token for
		Scope string `env:"GIGACHAT_SCOPE" envDefault:"GIGACHAT_API_PERS"`

		// Refresh token this many seconds before expiry
		TokenRefreshLeewaySeconds int `env:"GIGACHAT_TOKEN_REFRESH_LEEWAY_SECONDS" envDefault:"10"`

		// Basic auth token (base64 of client_id:client_secret) used to obtain OAuth access token
		BasicKey string `env:"GIGACHAT_BASIC_KEY,required"`

		// URL to a PEM-encoded Root CA certificate to trust for GigaChat API TLS
		RootCAURL string `env:"GIGACHAT_ROOT_CA_URL" envDefault:"https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt"`

		// Max tokens to request in chat completions
		MaxTokens int `env:"GIGACHAT_MAX_TOKENS" envDefault:"1024"`
	}
}

func isModelAllowed(model string) bool {
	switch model {
	case "GigaChat-2":
		return true
	case "GigaChat-2-Pro":
		return true
	case "GigaChat-2-Max":
		return true
	}
	return false
}

// Load loads .env (if present) and parses environment variables into Config.
func Load() (Config, error) {
	// Load .env if available; ignore error if file does not exist
	_ = godotenv.Load()

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}

	// Validate model value
	if !isModelAllowed(cfg.Gigachat.Model) {
		return Config{}, fmt.Errorf("invalid GIGACHAT_MODEL: %q (allowed: GigaChat-2, GigaChat-2-Pro, GigaChat-2-Max)", cfg.Gigachat.Model)
	}

	return cfg, nil
}
