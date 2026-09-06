// internal/site/cmd_test.go

package site_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smegg99/s99deploy/internal/site"
)

// config writes a config.json pointing at a binary that exists.
func config(t *testing.T, addr string) string {
	t.Helper()
	dir := t.TempDir()
	binary := filepath.Join(dir, "s99deploy")
	if err := os.WriteFile(binary, []byte("ELFBYTES"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.json")
	body := `{"gin_mode":"test","listen_addr":"` + addr + `","bin_path":"` + binary + `","trusted_proxies":["127.0.0.1"]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSiteExitCodes(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		code int
		says string
	}{
		{name: "help", args: []string{"--help"}, code: 0, says: "serve"},
		{name: "version", args: []string{"--version"}, code: 0, says: "s99deploy-site 1.0.0"},
		{name: "unknown flag", args: []string{"--nope"}, code: 2, says: "unknown flag"},
		{name: "unknown command", args: []string{"bogus"}, code: 2, says: "unknown command"},
		{name: "missing config", args: []string{"serve", "--config", "/nowhere/config.json"}, code: 1,
			says: "s99deploy-site:"},
	} {
		t.Run(c.name, func(t *testing.T) {
			out, errOut := &bytes.Buffer{}, &bytes.Buffer{}

			code := site.Main(context.Background(), out, errOut, c.args)

			if code != c.code {
				t.Errorf("code = %d, want %d\n%s%s", code, c.code, out, errOut)
			}
			if !strings.Contains(out.String()+errOut.String(), c.says) {
				t.Errorf("output does not carry %q:\n%s%s", c.says, out, errOut)
			}
		})
	}
}

// A signal is a clean stop for a server: it serves, it shuts down, it exits 0.
func TestServeStopsOnCancelAndExitsZero(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	path := config(t, addr)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan int, 1)
	go func() {
		done <- site.Main(ctx, &bytes.Buffer{}, &bytes.Buffer{}, []string{"serve", "--config", path})
	}()

	waitForGet(t, "http://"+addr+"/s99deploy.sha256")
	cancel()

	select {
	case code := <-done:
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not stop after the context was cancelled")
	}
}

func waitForGet(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never answered", url)
}
