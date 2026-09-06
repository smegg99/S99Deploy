// internal/site/routes.go

package site

import (
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

func (s *Server) usage(c *gin.Context) {
	c.String(http.StatusOK, usageText, s.baseURL(c))
}

func (s *Server) installScript(c *gin.Context) {
	c.String(http.StatusOK, installScript, s.baseURL(c))
}

func (s *Server) serveBinary(c *gin.Context) {
	c.FileAttachment(s.cfg.BinPath, "s99deploy")
}
