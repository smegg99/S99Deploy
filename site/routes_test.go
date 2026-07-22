package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func testRouter(t *testing.T, binPath string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router, err := buildRouter(&Config{
		BinPath:        binPath,
		TrustedProxies: []string{"127.0.0.1", "::1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func get(t *testing.T, router *gin.Engine, path string, forwardedProto string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "get.example.com"
	if forwardedProto != "" {
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
		req.RemoteAddr = "127.0.0.1:12345"
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestUsageTextPointsAtInstallScript(t *testing.T) {
	rec := get(t, testRouter(t, "unused"), "/", "https")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "curl -fsSL https://get.example.com/install.sh | sudo sh") {
		t.Errorf("usage missing install one-liner:\n%s", rec.Body.String())
	}
}

func TestInstallScriptUsesRequestHost(t *testing.T) {
	rec := get(t, testRouter(t, "unused"), "/install.sh", "https")
	body := rec.Body.String()
	if !strings.Contains(body, "curl -fsSL https://get.example.com/s99deploy -o /usr/local/bin/s99deploy") {
		t.Errorf("script missing download line:\n%s", body)
	}
	if !strings.HasPrefix(body, "#!/bin/sh") {
		t.Errorf("script missing shebang:\n%s", body)
	}
}

func TestServeBinary(t *testing.T) {
	binPath := filepath.Join(t.TempDir(), "s99deploy")
	if err := os.WriteFile(binPath, []byte("ELFBYTES"), 0o755); err != nil {
		t.Fatal(err)
	}
	rec := get(t, testRouter(t, binPath), "/s99deploy", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "ELFBYTES" {
		t.Errorf("body = %q", rec.Body.String())
	}
}
