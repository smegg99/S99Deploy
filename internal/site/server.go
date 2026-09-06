// internal/site/server.go

package site

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// Server serves the usage text, the install script and the binary.
type Server struct {
	cfg     *Config
	version string
	trusted proxySet
	router  *gin.Engine
}

// New builds the server and its routes.
func New(cfg *Config, version string) (*Server, error) {
	// A trusted_proxies entry neither this site nor gin can parse fails here.
	trusted, err := newProxySet(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}

	router := gin.New()
	router.Use(gin.Recovery())
	// gin's ClientIP and this site's scheme check are configured from one field,
	// so the two can never disagree.
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	s := &Server{cfg: cfg, version: version, trusted: trusted, router: router}
	router.GET("/", s.usage)
	router.GET("/install.sh", s.installScript)
	router.GET("/s99deploy", s.serveBinary)
	return s, nil
}

// Router is the handler, for a test that drives it without a listener.
func (s *Server) Router() *gin.Engine { return s.router }
