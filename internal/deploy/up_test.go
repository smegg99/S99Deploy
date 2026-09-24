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

	cfg, err := LiveConfig()
	if err != nil {
		t.Fatal(err)
	}
	prober := cfg.Prober
	code = http.StatusOK
	if err := prober.Probe(context.Background(), server.URL); err != nil {
		t.Errorf("200 was reported as unhealthy: %v", err)
	}
	code = http.StatusServiceUnavailable
	if err := prober.Probe(context.Background(), server.URL); err == nil {
		t.Error("503 was reported as healthy")
	}
}

func TestParseUnitStateReadsSystemctlShow(t *testing.T) {
	state := parseUnitState("ActiveState=active\nSubState=running\nResult=success\nInvocationID=8f2c\n")

	want := UnitState{Active: "active", Sub: "running", Result: "success", InvocationID: "8f2c"}
	if state != want {
		t.Errorf("parseUnitState = %+v, want %+v", state, want)
	}
}
