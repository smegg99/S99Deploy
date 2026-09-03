// internal/manifest/shipped_test.go

package manifest

import (
	"path/filepath"
	"testing"
)

// Every manifest this repo ships is loaded by the code an app's manifest is loaded by.
func TestShippedManifestsValidate(t *testing.T) {
	paths := []string{filepath.Join("..", "..", "deploy.json")}
	// Globbed, so a new example cannot be added without being validated.
	found, err := filepath.Glob(filepath.Join("..", "..", "examples", "*", "deploy.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths = append(paths, found...)

	if len(paths) < 3 {
		t.Fatalf("found %d manifests, want this repo's own plus its examples", len(paths))
	}
	for _, path := range paths {
		if _, err := Load(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}
