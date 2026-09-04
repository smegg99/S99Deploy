// internal/deploy/health_test.go

package deploy_test

import (
	"context"
	"errors"
	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/deploy/deploytest"
	"strings"
	"testing"
	"time"
)

// A crash loop must not be polled until the deadline.
func TestWaitHealthyFailsFastOnARestart(t *testing.T) {
	// The invocation id changes on every start, so a unit that restarted under
	// us is a failure, now.
	cfg, _, _, units, prober := deploytest.NewConfig(t)
	units.States["myapp"] = deploy.UnitState{Active: "active", Sub: "running", InvocationID: "second"}
	prober.Fail = 100

	started := time.Now()
	err := deploy.New(cfg).WaitHealthyForTest(context.Background(), "myapp",
		"http://127.0.0.1:1/", 30*time.Second,
		deploy.UnitState{Active: "active", InvocationID: "first"})

	if err == nil || !strings.Contains(err.Error(), "restarted") {
		t.Fatalf("err = %v, want it to report the restart", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("took %s, want the restart to be noticed at once", elapsed)
	}
}

func TestWaitHealthyFailsWhenTheUnitFailed(t *testing.T) {
	cfg, _, _, units, _ := deploytest.NewConfig(t)
	units.States["myapp"] = deploy.UnitState{Active: "failed", Result: "exit-code"}

	err := deploy.New(cfg).WaitHealthyForTest(context.Background(), "myapp",
		"http://127.0.0.1:1/", 30*time.Second, deploy.UnitState{InvocationID: "first"})

	if err == nil || !strings.Contains(err.Error(), "exit-code") {
		t.Fatalf("err = %v, want it to carry the unit's Result", err)
	}
}

// The probe is retried while the unit is healthy, which the fixed sleep stood in for.
func TestWaitHealthyRetriesUntilTheAppAnswers(t *testing.T) {
	cfg, _, _, units, prober := deploytest.NewConfig(t)
	units.States["myapp"] = deploy.UnitState{Active: "active", Sub: "running", InvocationID: "first"}
	prober.Fail = 3

	err := deploy.New(cfg).WaitHealthyForTest(context.Background(), "myapp",
		"http://127.0.0.1:1/", 30*time.Second, deploy.UnitState{InvocationID: "first"})

	if err != nil {
		t.Fatalf("waitHealthy: %v", err)
	}
	if prober.Calls != 4 {
		t.Errorf("probed %d times, want 4", prober.Calls)
	}
}

func TestWaitHealthyStopsAtCancel(t *testing.T) {
	cfg, _, _, units, prober := deploytest.NewConfig(t)
	units.States["myapp"] = deploy.UnitState{Active: "active", Sub: "running", InvocationID: "first"}
	prober.Fail = 1000
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()
	err := deploy.New(cfg).WaitHealthyForTest(ctx, "myapp", "http://127.0.0.1:1/",
		30*time.Second, deploy.UnitState{InvocationID: "first"})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want it to wrap context.Canceled", err)
	}
}

type deadlineProber struct{}

func (deadlineProber) Probe(ctx context.Context, _ string) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestHealthTimeoutBoundsAnInFlightProbe(t *testing.T) {
	cfg, _, _, units, _ := deploytest.NewConfig(t)
	cfg.Prober = deadlineProber{}
	units.States["myapp"] = deploy.UnitState{Active: "active", InvocationID: "first"}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := deploy.New(cfg).WaitHealthyForTest(ctx, "myapp", "http://127.0.0.1:1/", 25*time.Millisecond, units.States["myapp"])
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("25ms health timeout took %s", elapsed)
	}
}

func TestUpBuildsRestartsAndProbesWithoutRoot(t *testing.T) {
	cfg, runner, _, units, prober := deploytest.NewConfig(t)
	clones(t, runner, nil)
	d := deploy.New(cfg)
	if _, err := d.Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}
	units.States["myapp"] = deploy.UnitState{Active: "active", InvocationID: "first"}
	start := time.Now()
	if err := d.Up(context.Background(), "myapp", time.Second); err != nil {
		t.Fatal(err)
	}
	if units.Restarts != 1 || prober.Calls != 1 {
		t.Fatalf("restarts=%d probes=%d", units.Restarts, prober.Calls)
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Fatalf("healthy deployment delayed %s", elapsed)
	}
	var pulled, built bool
	for _, call := range runner.Calls {
		pulled = pulled || call.Command == "git pull --ff-only"
		built = built || call.Command == "go build -o bin/myapp ."
	}
	if !pulled || !built {
		t.Fatalf("pulled=%t built=%t", pulled, built)
	}
}

func TestUpDoesNotRestartAfterACancelledBuild(t *testing.T) {
	cfg, runner, _, units, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	d := deploy.New(cfg)
	if _, err := d.Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}
	runner.Fail["go build -o bin/myapp ."] = context.Canceled
	err := d.Up(context.Background(), "myapp", time.Second)
	if !errors.Is(err, context.Canceled) || units.Restarts != 0 {
		t.Fatalf("error=%v restarts=%d", err, units.Restarts)
	}
}
