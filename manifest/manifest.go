// manifest/manifest.go

// Package manifest loads and validates deploy.json, the per-app deploy
// definition each app repo carries in its root.
package manifest

import (
	_ "embed"
	"fmt"

	"github.com/smegg99/s99config"
)

//go:generate bash ../scripts/gen-types.sh

//go:embed schema.cue
var schema []byte

// Load reads, validates against the CUE schema, and decodes the manifest.
func Load(path string) (*Manifest, error) {
	loader, err := s99config.New(schema, s99config.WithDefinition("#Manifest"))
	if err != nil {
		return nil, fmt.Errorf("compile manifest schema: %w", err)
	}
	if err := loader.Load(path); err != nil {
		return nil, fmt.Errorf("load manifest %s: %w", path, err)
	}
	var m Manifest
	if err := loader.Decode(&m); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	return &m, nil
}
