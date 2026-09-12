// internal/site/proxy_test.go

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

// newServer is a site that trusts the loopback and nothing else.
func newServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	binPath := filepath.Join(t.TempDir(), "s99deploy")
	if err := os.WriteFile(binPath, []byte("ELFBYTES"), 0o755); err != nil {
		t.Fatal(err)
	}
	server, err := New(&Config{BinPath: binPath, TrustedProxies: []string{"127.0.0.1"}}, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return server
}

func TestBaseURLBelievesOnlyATrustedPeer(t *testing.T) {
	server := newServer(t)

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

			got, ok := server.baseURL(ctx)
			if !ok {
				t.Fatalf("baseURL refused host %q", req.Host)
			}
			if got != c.want {
				t.Errorf("baseURL = %q, want %q", got, c.want)
			}
		})
	}
}

// gin reads a plain mapped address as ::/32; this site refuses every mapped spelling.
func TestProxySetRefusesAMappedAddressConfiguration(t *testing.T) {
	// One field feeds both, so an entry the two read differently is a config
	// error, not something for one of them to quietly reinterpret. The refusal
	// names the plain spelling in full, prefix length included.
	for _, c := range []struct{ entry, want string }{
		{entry: "::ffff:10.4.2.1", want: "10.4.2.1"},
		{entry: "::ffff:10.0.0.0/104", want: "10.0.0.0/8"},
	} {
		_, err := newProxySet([]string{c.entry})
		if err == nil {
			t.Fatalf("%s was accepted", c.entry)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("err = %v, want it to name %s", err, c.want)
		}
	}
}

// An IPv6 zone id is refused, because gin cannot parse one and would disagree.
func TestProxySetRefusesAZonedEntry(t *testing.T) {
	for _, entry := range []string{"fe80::1%eth0", "fe80::%eth0/64"} {
		if _, err := newProxySet([]string{entry}); err == nil {
			t.Errorf("%s was accepted", entry)
		}
	}
}

// The plain spelling of the same address is what both parsers agree on.
func TestProxySetTrustsThePlainSpelling(t *testing.T) {
	set, err := newProxySet([]string{"10.4.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	if !set.has("10.4.2.1") || !set.has("::ffff:10.4.2.1") {
		t.Error("10.4.2.1 does not trust its own peer")
	}
}

// The install script is read by root, so the host it quotes is checked first.
func TestBaseURLRefusesAHostTheScriptCannotQuote(t *testing.T) {
	// net/http rejects a backtick, a quote and a backslash, and lets $ ( ) '
	// and ; through. Inside the script's url="..." those still expand.
	server := newServer(t)
	for _, host := range []string{
		"x$(touch /tmp/pwned)y", "x$(id)y", "x'y", "x$IFSy", "x;idy", "x&y", "", "x/y",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req

		if base, ok := server.baseURL(ctx); ok {
			t.Errorf("host %q was accepted as %q", host, base)
		}
	}
}

func TestBaseURLTakesTheHostsAProxyActuallySends(t *testing.T) {
	server := newServer(t)
	for _, host := range []string{
		"get.example.com", "get.example.com:9250", "127.0.0.1:9250", "[::1]:9250", "localhost",
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = req

		base, ok := server.baseURL(ctx)
		if !ok {
			t.Errorf("host %q was refused", host)
			continue
		}
		if !strings.HasSuffix(base, "://"+host) {
			t.Errorf("baseURL = %q, want it to end in %q", base, host)
		}
	}
}
