// internal/deploy/up_test.go

package deploy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The live prober is the only piece of the health check that talks to a socket.
func TestLiveProberReadsTheStatusCode(t *testing.T) {
	var code int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	defer server.Close()

	prober := LiveConfig().Prober
	code = http.StatusOK
	if err := prober.Probe(context.Background(), server.URL); err != nil {
		t.Errorf("200 was reported as unhealthy: %v", err)
	}
	code = http.StatusServiceUnavailable
	if err := prober.Probe(context.Background(), server.URL); err == nil {
		t.Error("503 was reported as healthy")
	}
}
