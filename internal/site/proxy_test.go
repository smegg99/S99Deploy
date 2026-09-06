// internal/site/proxy_test.go

package site

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewProxySetTakesAddressesAndBlocks(t *testing.T) {
	set, err := newProxySet([]string{"127.0.0.1", "::1", "10.0.0.0/8", "2001:db8::/32"})
	if err != nil {
		t.Fatal(err)
	}

	for ip, want := range map[string]bool{
		"127.0.0.1":       true,
		"127.0.0.2":       false,
		"::1":             true,
		"10.4.2.1":        true,
		"11.0.0.1":        false,
		"2001:db8::dead":  true,
		"2001:dbf::dead":  false,
		"::ffff:10.4.2.1": true,
		"not-an-address":  false,
	} {
		if got := set.has(ip); got != want {
			t.Errorf("has(%q) = %v, want %v", ip, got, want)
		}
	}
}

func TestNewProxySetRejectsNonsense(t *testing.T) {
	if _, err := newProxySet([]string{"127.0.0.1", "gateway"}); err == nil {
		t.Fatal("a hostname was accepted as a trusted proxy")
	}
}

func TestEmptyProxySetTrustsNobody(t *testing.T) {
	set, err := newProxySet(nil)
	if err != nil {
		t.Fatal(err)
	}
	if set.has("127.0.0.1") {
		t.Error("an empty trusted_proxies trusted loopback")
	}
}

func TestForwardedScheme(t *testing.T) {
	for header, want := range map[string]string{
		"https":       "https",
		"HTTPS":       "https",
		" http ":      "http",
		"https, http": "https",
		"javascript:": "",
		"":            "",
		"ftp":         "",
	} {
		if got := forwardedScheme(header); got != want {
			t.Errorf("forwardedScheme(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestBaseURLBelievesOnlyATrustedPeer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server, err := New(&Config{BinPath: "unused", TrustedProxies: []string{"127.0.0.1"}}, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name, peer, forwarded, want string
	}{
		{"trusted peer", "127.0.0.1:9999", "https", "https://get.example.com"},
		{"untrusted peer", "203.0.113.9:9999", "https", "http://get.example.com"},
		{"trusted peer, no header", "127.0.0.1:9999", "", "http://get.example.com"},
		{"trusted peer, junk header", "127.0.0.1:9999", "gopher", "http://get.example.com"},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = "get.example.com"
			req.RemoteAddr = c.peer
			if c.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", c.forwarded)
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = req

			if got := server.baseURL(ctx); got != c.want {
				t.Errorf("baseURL = %q, want %q", got, c.want)
			}
		})
	}
}

func TestProxySetAcceptsMappedAddressConfiguration(t *testing.T) {
	for _, entry := range []string{"::ffff:10.4.2.1", "::ffff:10.0.0.0/104"} {
		set, err := newProxySet([]string{entry})
		if err != nil {
			t.Fatal(err)
		}
		if !set.has("10.4.2.1") || !set.has("::ffff:10.4.2.1") {
			t.Errorf("%s does not trust its configured address", entry)
		}
	}
}
