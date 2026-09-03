// internal/site/config.go

package site

import (
	_ "embed"
	"fmt"

	"github.com/smegg99/s99config"
)

//go:embed schema.cue
var schema []byte

// LoadConfig validates and decodes the site config at path.
func LoadConfig(path string) (*Config, error) {
	loader, err := s99config.New(schema)
	if err != nil {
		return nil, fmt.Errorf("compile config schema: %w", err)
	}
	if err := loader.Load(path); err != nil {
		return nil, fmt.Errorf("load config %s: %w", path, err)
	}
	var cfg Config
	if err := loader.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config %s: %w", path, err)
	}
	return &cfg, nil
}
