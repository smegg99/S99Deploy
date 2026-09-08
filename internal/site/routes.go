// internal/site/routes.go

package site

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func (s *Server) usage(c *gin.Context) {
	base, ok := s.baseURL(c)
	if !ok {
		refuseHost(c)
		return
	}
	c.String(http.StatusOK, usageText, base, s.version)
}

func (s *Server) installScript(c *gin.Context) {
	base, ok := s.baseURL(c)
	if !ok {
		refuseHost(c)
		return
	}
	sum, _, err := s.binary.get()
	if err != nil {
		c.String(http.StatusServiceUnavailable, "s99deploy: %s is not readable\n", s.cfg.BinPath)
		return
	}
	c.String(http.StatusOK, installScript, base, s.version, sum)
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

// refuseHost answers a host the served commands will not quote.
func refuseHost(c *gin.Context) {
	// The host is not echoed. This body can reach a root shell through
	// `curl ... | sudo sh`, so it carries nothing the request chose.
	c.String(http.StatusBadRequest, "s99deploy: this site does not answer to that host\n")
}
