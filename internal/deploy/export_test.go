// internal/deploy/export_test.go

package deploy

import (
	"context"
	"time"
)

// WaitHealthyForTest exposes the health wait to the flow tests.
func (d *Deployer) WaitHealthyForTest(ctx context.Context, name, url string, timeout time.Duration, baseline UnitState) error {
	return d.waitHealthy(ctx, name, url, timeout, baseline)
}

// AsUserEnvForTest is the environment the live runner hands a build command.
func AsUserEnvForTest(a AsUser, base []string) []string { return asUserEnv(a, base) }

// LastEnvValueForTest reads a key the way os/exec does: the last entry wins.
func LastEnvValueForTest(env []string, key string) string { return lastValue(env, key) }
