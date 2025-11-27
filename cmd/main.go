package main

import (
	"fmt"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog/log"

	"pod_api/pkg/api"
	openapi "pod_api/pkg/apigen/openapi"
	"pod_api/pkg/clients/gigachat"
	"pod_api/pkg/clients/openai"
	"pod_api/pkg/config"
	"pod_api/pkg/logging"
	"pod_api/pkg/metrics"
	appmw "pod_api/pkg/middleware"
	imagerepo "pod_api/pkg/repository/image"
)

func main() {
	// Setup logging
	logging.Setup()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config failed")
	}

	// Observability pieces
	reg := metrics.NewRegistry()

	server := echo.New()
	server.HideBanner = true
	server.Use(echomw.Recover())
	server.Use(appmw.RequestLogger(reg))

	// Healthcheck and metrics
	server.GET("/ping", func(c echo.Context) error { return c.String(200, "pong") })
	server.GET("/metrics", reg.EchoHandlerText)
	server.GET("/metrics.json", reg.EchoHandlerJSON)

	gigachatClient, err := gigachat.NewFromConfig(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("gigachat client init failed")
	}

	openaiClient, err := openai.NewClient(
		cfg.OpenAI.BasicKey,
		cfg.OpenAI.URL,
		cfg.OpenAI.Model,
		cfg.OpenAI.RequestTimeout,
	)

	if err != nil {
		log.Fatal().Err(err).Msg("openai client init failed")
	}

	imageRepository := imagerepo.NewMemoryRepository(reg)
	handlers, err := api.NewHandlers(
		gigachatClient,
		openaiClient,
		imageRepository,
		cfg.Server.BaseURL,
		cfg.ImageTTL,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create handlers")
	}
	openapi.RegisterHandlers(server, openapi.NewStrictHandler(handlers, nil))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Info().Str("addr", addr).Msg("starting server")
	if err := server.Start(addr); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}
