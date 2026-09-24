// internal/deploy/events.go

package deploy

import "github.com/smegg99/s99deploy/internal/messages"

// The log event ids, each a catalog key the logger's translator renders.
const (
	EventCloning         = messages.KeyLogsCloning
	EventCheckoutPresent = messages.KeyLogsCheckoutPresent
	EventCreatedEnv      = messages.KeyLogsCreatedEnv
	EventInstalled       = messages.KeyLogsInstalled
	EventPulling         = messages.KeyLogsPulling
	EventBuilding        = messages.KeyLogsBuilding
	EventRestarting      = messages.KeyLogsRestarting
	EventDeployed        = messages.KeyLogsDeployed
	EventPurged          = messages.KeyLogsPurged
	EventUninstalled     = messages.KeyLogsUninstalled
)
