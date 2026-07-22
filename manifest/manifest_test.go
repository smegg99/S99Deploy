package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "deploy.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAppliesDefaults(t *testing.T) {
	m, err := Load(write(t, `{
		"name": "smeggtunersite",
		"build": ["go build -o bin/app ."],
		"run": ["bin/app"],
		"check": {"port": 9245}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "smeggtunersite" {
		t.Errorf("name = %q", m.Name)
	}
	if m.Check.Port != 9245 {
		t.Errorf("port = %d", m.Check.Port)
	}
	if m.Check.Path != "/" {
		t.Errorf("default path = %q", m.Check.Path)
	}
	if m.Check.TimeoutSeconds != 10 {
		t.Errorf("default timeout = %d", m.Check.TimeoutSeconds)
	}
	if len(m.Env) != 0 || len(m.RequireEnv) != 0 {
		t.Errorf("env defaults not empty: %v %v", m.Env, m.RequireEnv)
	}
}

func TestLoadRejectsInvalid(t *testing.T) {
	cases := map[string]string{
		"uppercase name": `{"name": "Bad", "run": ["x"], "check": {"port": 1}}`,
		"empty run":      `{"name": "ok", "run": [], "check": {"port": 1}}`,
		"empty argv0":    `{"name": "ok", "run": [""], "check": {"port": 1}}`,
		"port zero":      `{"name": "ok", "run": ["x"], "check": {"port": 0}}`,
		"port too high":  `{"name": "ok", "run": ["x"], "check": {"port": 70000}}`,
		"missing check":  `{"name": "ok", "run": ["x"]}`,
		"unknown field":  `{"name": "ok", "run": ["x"], "check": {"port": 1}, "wat": true}`,
	}
	for label, doc := range cases {
		if _, err := Load(write(t, doc)); err == nil {
			t.Errorf("%s: expected validation error", label)
		}
	}
}
