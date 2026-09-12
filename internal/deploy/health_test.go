// internal/deploy/health_test.go

package deploy_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/deploy/deploytest"
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
	// An interrupt is not a health verdict, so it passes through unwrapped.
	if strings.Contains(err.Error(), "did not become healthy") {
		t.Errorf("err = %v, want the bare cancellation", err)
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
	// The wait's own deadline is a failed health check, so it says which app
	// and how long it waited. Only a cancelled caller passes through unwrapped.
	if !strings.Contains(err.Error(), "myapp did not become healthy within 25ms") {
		t.Errorf("error = %v, want it to name the app and the timeout", err)
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

// A name that is not manifest-shaped is refused before Up touches a path.
func TestUpRefusesABadName(t *testing.T) {
	cfg, runner, _, _, _ := deploytest.NewConfig(t)
	for _, name := range []string{"../x", "/", "-x", "", "a/b"} {
		if err := deploy.New(cfg).Up(context.Background(), name, time.Second); err == nil {
			t.Errorf("Up(%q) was allowed", name)
		}
	}
	if len(runner.Calls) != 0 {
		t.Errorf("a bad name still ran %v", runner.Calls)
	}
}

// git reads .gitconfig and .ssh from HOME, which is the account's, not /opt/<name>.
func TestUpRunsWithTheAccountsOwnHome(t *testing.T) {
	cfg, runner, accounts, units, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	d := deploy.New(cfg)
	if _, err := d.Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}
	// A home that is not the app root, which is what the old code passed.
	account := accounts.Users["myapp"]
	account.Home = filepath.Join(t.TempDir(), "home", "myapp")
	accounts.Users["myapp"] = account
	units.States["myapp"] = deploy.UnitState{Active: "active", InvocationID: "first"}

	if err := d.Up(context.Background(), "myapp", time.Second); err != nil {
		t.Fatal(err)
	}

	var seen int
	for _, call := range runner.Calls {
		if call.As.Name == "" {
			continue
		}
		seen++
		if call.As.Home != account.Home {
			t.Errorf("%q ran with HOME=%s, want %s", call.Command, call.As.Home, account.Home)
		}
	}
	if seen == 0 {
		t.Fatal("nothing ran as the account")
	}
}
