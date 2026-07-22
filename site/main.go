// site/main.go

// The curl-able home of the s99deploy binary. Serves usage text at /, an
// install script at /install.sh, and the binary itself at /s99deploy, so any
// VPS can bootstrap with one command.
package main

import (
	"os"

	"github.com/gin-gonic/gin"
	"github.com/smegg99/s99logger"
)

func main() {
	s99logger.SetDefault(s99logger.New(
		s99logger.NewConsoleSink(os.Stderr),
		s99logger.Options{Service: "s99deploy-site"},
	))

	cfg, err := loadConfig()
	if err != nil {
		s99logger.Fatal(s99logger.NewEvent("config_failed", s99logger.Err(err)))
	}
	gin.SetMode(cfg.GinMode)

	router, err := buildRouter(cfg)
	if err != nil {
		s99logger.Fatal(s99logger.NewEvent("router_failed", s99logger.Err(err)))
	}

	s99logger.Info(s99logger.NewEvent("listening", s99logger.String("addr", cfg.ListenAddr)))
	if err := router.Run(cfg.ListenAddr); err != nil {
		s99logger.Fatal(s99logger.NewEvent("server_failed", s99logger.Err(err)))
	}
}
