// internal/site/install_test.go

package site

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// serving is a site over a binary with known bytes.
func serving(t *testing.T, body string) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "s99deploy")
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	server, err := New(&Config{BinPath: path, TrustedProxies: []string{"127.0.0.1"}}, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return server.Router(), path
}

func fetch(t *testing.T, router *gin.Engine, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Host = "get.example.com"
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// The script, the sha256 endpoint and the served bytes have to agree.
func TestInstallScriptQuotesTheServedDigest(t *testing.T) {
	router, _ := serving(t, "ELFBYTES")
	want := sum(t, "ELFBYTES")

	script := fetch(t, router, "/install.sh").Body.String()
	if !strings.Contains(script, `want="`+want+`"`) {
		t.Errorf("install.sh does not quote the digest %s:\n%s", want, script)
	}
	if !strings.HasPrefix(script, "#!/bin/sh") {
		t.Errorf("install.sh has no shebang:\n%s", script)
	}
	if !strings.Contains(script, "https://get.example.com/s99deploy") {
		t.Errorf("install.sh does not point at the request host:\n%s", script)
	}
	// A truncated download of the script itself must define a function and do
	// nothing, which is what the trailing call buys.
	if !strings.HasSuffix(strings.TrimSpace(script), `main "$@"`) {
		t.Errorf("install.sh does not end with its own invocation:\n%s", script)
	}

	checksum := fetch(t, router, "/s99deploy.sha256").Body.String()
	if checksum != want+"  s99deploy\n" {
		t.Errorf("/s99deploy.sha256 = %q, want the sha256sum format", checksum)
	}

	binary := fetch(t, router, "/s99deploy")
	if binary.Body.String() != "ELFBYTES" {
		t.Errorf("/s99deploy served %q", binary.Body.String())
	}
}

func TestUsageTextPointsAtTheInstallScript(t *testing.T) {
	router, _ := serving(t, "ELFBYTES")

	body := fetch(t, router, "/").Body.String()
	if !strings.Contains(body, "curl -fsSL https://get.example.com/install.sh | sudo sh") {
		t.Errorf("usage is missing the install one-liner:\n%s", body)
	}
	if !strings.Contains(body, "1.0.0") {
		t.Errorf("usage does not say which version it serves:\n%s", body)
	}
}

// A site that cannot find its binary fails the deploy's own health check.
func TestNewRefusesAMissingBinary(t *testing.T) {
	_, err := New(&Config{BinPath: filepath.Join(t.TempDir(), "gone")}, "1.0.0")

	if err == nil {
		t.Fatal("a site with no binary started")
	}
	if !strings.Contains(err.Error(), "gone") {
		t.Errorf("err = %v, want it to name the path", err)
	}
}
