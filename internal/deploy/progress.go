// internal/deploy/progress.go

package deploy

// Step is one stage of work a reader waits on.
type Step string

// The stages the flows report.
const (
	StepClone   Step = "cloning"
	StepPull    Step = "pulling"
	StepBuild   Step = "building"
	StepRestart Step = "restarting"
	StepHealth  Step = "health_check"
)

// Progress reports stage boundaries; Begin returns the function that ends one.
type Progress interface {
	Begin(step Step, detail string) (end func(error))
}

// noProgress is the default, so the flows never check for nil.
type noProgress struct{}

func (noProgress) Begin(Step, string) func(error) { return func(error) {} }
