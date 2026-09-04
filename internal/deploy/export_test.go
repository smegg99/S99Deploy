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
