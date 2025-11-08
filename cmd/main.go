package main

import (
	"fmt"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"pod_api/pkg/api"
	openapi "pod_api/pkg/apigen/openapi"
	"pod_api/pkg/clients/gigachat"
	"pod_api/pkg/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config failed: %v", err)
	}

	server := echo.New()
	server.HideBanner = true
	server.Use(middleware.Recover())
	server.Use(middleware.Logger())

	gigachatClient, err := gigachat.NewFromConfig(cfg)
	if err != nil {
		log.Fatalf("gigachat client init failed: %v", err)
	}

	handlers := api.NewHandlers(gigachatClient, nil)
	openapi.RegisterHandlers(server, openapi.NewStrictHandler(handlers, nil))

	if err := server.Start(fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
