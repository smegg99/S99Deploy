// site/routes.go

package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

const usageText = `s99deploy -- deploy manifest-carrying apps to /opt under systemd

Install on a VPS:

  curl -fsSL %[1]s/install.sh | sudo sh

Or by hand:

  sudo curl -fsSL %[1]s/s99deploy -o /usr/local/bin/s99deploy
  sudo chmod 0755 /usr/local/bin/s99deploy

Docs: https://github.com/smegg99/S99Deploy
`

const installScript = `#!/bin/sh
set -eu
curl -fsSL %[1]s/s99deploy -o /usr/local/bin/s99deploy
chmod 0755 /usr/local/bin/s99deploy
echo "installed /usr/local/bin/s99deploy"
`

// buildRouter wires the three endpoints: usage text, the install script, and
// the binary itself.
func buildRouter(cfg *Config) (*gin.Engine, error) {
	router := gin.New()
	router.Use(gin.Recovery())
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, usageText, baseURL(c))
	})
	router.GET("/install.sh", func(c *gin.Context) {
		c.String(http.StatusOK, installScript, baseURL(c))
	})
	router.GET("/s99deploy", func(c *gin.Context) {
		c.FileAttachment(cfg.BinPath, "s99deploy")
	})

	return router, nil
}

// baseURL reconstructs the public URL for the text endpoints so the printed
// commands point back at whatever host served them. X-Forwarded-Proto is only
// honored from trusted proxies (gin blanks untrusted forwarded headers).
func baseURL(c *gin.Context) string {
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + c.Request.Host
}
