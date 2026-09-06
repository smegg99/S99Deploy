// internal/site/routes.go

package site

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s *Server) usage(c *gin.Context) {
	c.String(http.StatusOK, usageText, s.baseURL(c), s.version)
}

func (s *Server) installScript(c *gin.Context) {
	sum, _, err := s.binary.get()
	if err != nil {
		c.String(http.StatusServiceUnavailable, "s99deploy: %s is not readable\n", s.cfg.BinPath)
		return
	}
	c.String(http.StatusOK, installScript, s.baseURL(c), s.version, sum)
}

// checksum is the sha256sum file format: the digest, two spaces, the name.
func (s *Server) checksum(c *gin.Context) {
	sum, _, err := s.binary.get()
	if err != nil {
		c.String(http.StatusServiceUnavailable, "s99deploy: %s is not readable\n", s.cfg.BinPath)
		return
	}
	c.String(http.StatusOK, "%s  s99deploy\n", sum)
}

func (s *Server) serveBinary(c *gin.Context) {
	c.FileAttachment(s.cfg.BinPath, "s99deploy")
}
