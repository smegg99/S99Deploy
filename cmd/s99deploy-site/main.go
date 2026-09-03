// cmd/s99deploy-site/main.go

// s99deploy-site serves the s99deploy binary, its checksum and its install script.
package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/site"
)

func main() {
	s99logger.SetDefault(s99logger.New(
		s99logger.NewConsoleSink(os.Stderr),
		s99logger.Options{Service: "s99deploy-site"},
	))

	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config.json"
	}
	cfg, err := site.LoadConfig(path)
	if err != nil {
		s99logger.Fatal(s99logger.NewEvent("config_failed", s99logger.Err(err)))
	}
	gin.SetMode(cfg.GinMode)

	router, err := site.BuildRouter(cfg)
	if err != nil {
		s99logger.Fatal(s99logger.NewEvent("router_failed", s99logger.Err(err)))
	}

	s99logger.Info(s99logger.NewEvent("listening", s99logger.String("addr", cfg.ListenAddr)))
	if err := router.Run(cfg.ListenAddr); err != nil {
		s99logger.Fatal(s99logger.NewEvent("server_failed", s99logger.Err(err)))
	}
}
