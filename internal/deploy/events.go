// internal/deploy/events.go

package deploy

import "github.com/smegg99/s99deploy/internal/messages"

// The log event ids, each a catalog key the logger's translator renders.
const (
	EventCheckoutPresent = messages.KeyLogsCheckoutPresent
	EventCreatedEnv      = messages.KeyLogsCreatedEnv
	EventInstalled       = messages.KeyLogsInstalled
	EventDeployed        = messages.KeyLogsDeployed
	EventPurged          = messages.KeyLogsPurged
	EventUninstalled     = messages.KeyLogsUninstalled
)
