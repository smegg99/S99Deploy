// deploy/env.go

package deploy

import (
	"fmt"
	"sort"
	"strings"
)

// BuildEnv merges base "K=V" pairs (usually os.Environ) with overlays in
// order; later values win.
func BuildEnv(base []string, overlays ...map[string]string) map[string]string {
	env := make(map[string]string, len(base))
	for _, kv := range base {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	for _, overlay := range overlays {
		for k, v := range overlay {
			env[k] = v
		}
	}
	return env
}

// PrependPath puts dir in front of env's PATH.
func PrependPath(env map[string]string, dir string) {
	if current := env["PATH"]; current != "" {
		env["PATH"] = dir + ":" + current
		return
	}
	env["PATH"] = dir
}

// CheckRequired reports every required var that is unset or empty, so one
// failed deploy surfaces the whole list instead of one name per attempt.
func CheckRequired(env map[string]string, required []string) error {
	var missing []string
	for _, name := range required {
		if env[name] == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required env vars unset or empty: %s", strings.Join(missing, ", "))
	}
	return nil
}

// EnvSlice flattens env to sorted "K=V" pairs for exec.
func EnvSlice(env map[string]string) []string {
	pairs := make([]string, 0, len(env))
	for k, v := range env {
		pairs = append(pairs, k+"="+v)
	}
	sort.Strings(pairs)
	return pairs
}
