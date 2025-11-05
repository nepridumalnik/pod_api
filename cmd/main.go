package main

import (
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"pod_api/pkg/api"
	"pod_api/pkg/apigen"
	"pod_api/pkg/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	e.Use(middleware.Logger())

	// Register strict server
	handlers := &api.Handlers{}
	apigen.RegisterHandlers(e, apigen.NewStrictHandler(handlers, nil))

	if err := e.Start(cfg.Address); err != nil {
		log.Fatalf("server: %v", err)
	}
}
