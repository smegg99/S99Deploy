// internal/site/server_test.go

package site

import (
	"context"
	"net"
	"runtime"
	"strings"
	"testing"
	"time"
)

func serveWaiters() int {
	stack := make([]byte, 1<<20)
	size := runtime.Stack(stack, true)
	return strings.Count(string(stack[:size]), "github.com/smegg99/s99deploy/internal/site.(*Server).Serve.func1")
}

func TestServeReleasesShutdownWaiterAfterListenFailure(t *testing.T) {
	_, path := serving(t, "ELFBYTES")
	server, err := New(&Config{BinPath: path}, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	before := serveWaiters()
	for range 5 {
		if err := server.Serve(ctx, listener.Addr().String()); err == nil {
			t.Fatal("occupied address was accepted")
		}
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if serveWaiters() <= before {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("failed starts left %d shutdown goroutines", serveWaiters()-before)
}
