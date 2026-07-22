// site/config.go

package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/smegg99/s99config"
)

//go:embed schema.cue
var schema []byte

// loadConfig validates and decodes the site config from CONFIG_PATH, falling
// back to config.json in the working directory.
func loadConfig() (*Config, error) {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "config.json"
	}
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
