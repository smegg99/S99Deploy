// internal/site/server.go

package site

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Server serves the usage text, the install script, the digest and the binary.
type Server struct {
	cfg     *Config
	version string
	trusted proxySet
	binary  *binDigest
	router  *gin.Engine
}

// New builds the server and its routes.
func New(cfg *Config, version string) (*Server, error) {
	trusted, err := newProxySet(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}

	binary := &binDigest{path: cfg.BinPath}
	// A site that cannot find its binary fails the deploy's health check now,
	// instead of serving a 500 to whoever curls it next.
	if _, _, err := binary.get(); err != nil {
		return nil, fmt.Errorf("bin_path %s: %w", cfg.BinPath, err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	// gin's ClientIP and this site's scheme check are configured from one field,
	// so the two can never disagree.
	if err := router.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		return nil, fmt.Errorf("set trusted proxies: %w", err)
	}

	s := &Server{cfg: cfg, version: version, trusted: trusted, binary: binary, router: router}
	router.GET("/", s.usage)
	router.GET("/install.sh", s.installScript)
	router.GET("/s99deploy", s.serveBinary)
	router.GET("/s99deploy.sha256", s.checksum)
	return s, nil
}

// Router is the handler, for a test that drives it without a listener.
func (s *Server) Router() *gin.Engine { return s.router }

// Serve listens until ctx is cancelled, then drains in-flight requests.
func (s *Server) Serve(ctx context.Context, addr string) error {
	// A signal is a clean stop for a server, so this returns nil.
	// The cancel releases the waiter below when the listen itself fails.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	server := &http.Server{Addr: addr, Handler: s.router}
	idle := make(chan error, 1)

	go func() {
		<-ctx.Done()
		stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		idle <- server.Shutdown(stop)
	}()

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return <-idle
}
