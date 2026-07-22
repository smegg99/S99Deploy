package manifest

import "testing"

// The repo deploys itself (the site that hosts the CLI binary), so its own
// manifest and the shipped examples must always validate.
func TestRepoManifestsValidate(t *testing.T) {
	for _, path := range []string{
		"../deploy.json",
		"../examples/minimal/deploy.json",
		"../examples/webapp/deploy.json",
	} {
		if _, err := Load(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}
