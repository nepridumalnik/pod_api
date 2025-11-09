package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"pod_api/pkg/api"
	openapi "pod_api/pkg/apigen/openapi"
	"pod_api/pkg/clients/gigachat"
	"pod_api/pkg/clients/openai"
	"pod_api/pkg/config"
)

func main() {
	config, err := config.Load()
	if err != nil {
		log.Fatalf("config failed: %v", err)
	}

	server := echo.New()
	server.HideBanner = true
	server.Use(middleware.Recover())
	server.Use(middleware.Logger())

	gigachatClient, err := gigachat.NewFromConfig(config)
	if err != nil {
		log.Fatalf("gigachat client init failed: %v", err)
	}

	openaiClient, err := openai.NewClient(config.OpenAI.BasicKey, config.OpenAI.URL, config.OpenAI.Model)

	if err != nil {
		log.Fatalf("openai client init failed: %v", err)
	}

	handlers, err := api.NewHandlers(gigachatClient, openaiClient)
	if err != nil {
		log.Fatalf("failed to create handlers: %v", err)
	}
	openapi.RegisterHandlers(server, openapi.NewStrictHandler(handlers, nil))

	if err := server.Start(fmt.Sprintf("%s:%d", config.Server.Host, config.Server.Port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
