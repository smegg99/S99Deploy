// internal/deploy/events.go

package deploy

// The log event ids, each a catalog key the logger's translator renders.
const (
	EventCloning         = "logs.cloning"
	EventCheckoutPresent = "logs.checkoutPresent"
	EventCreatedEnv      = "logs.createdEnv"
	EventInstalled       = "logs.installed"
	EventPulling         = "logs.pulling"
	EventBuilding        = "logs.building"
	EventRestarting      = "logs.restarting"
	EventDeployed        = "logs.deployed"
	EventPurged          = "logs.purged"
	EventUninstalled     = "logs.uninstalled"
)
