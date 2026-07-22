package deploy

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildEnvOverlayOrder(t *testing.T) {
	env := BuildEnv(
		[]string{"A=base", "B=base", "PATH=/usr/bin"},
		map[string]string{"B": "dotenv", "C": "dotenv"},
		map[string]string{"C": "manifest"},
	)
	want := map[string]string{"A": "base", "B": "dotenv", "C": "manifest", "PATH": "/usr/bin"}
	if !reflect.DeepEqual(env, want) {
		t.Errorf("got %v, want %v", env, want)
	}
}

func TestPrependPath(t *testing.T) {
	env := map[string]string{"PATH": "/usr/bin"}
	PrependPath(env, "/usr/local/go/bin")
	if env["PATH"] != "/usr/local/go/bin:/usr/bin" {
		t.Errorf("PATH = %q", env["PATH"])
	}
	empty := map[string]string{}
	PrependPath(empty, "/usr/local/go/bin")
	if empty["PATH"] != "/usr/local/go/bin" {
		t.Errorf("PATH = %q", empty["PATH"])
	}
}

func TestCheckRequired(t *testing.T) {
	env := map[string]string{"SET": "x", "EMPTY": ""}
	if err := CheckRequired(env, []string{"SET"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err := CheckRequired(env, []string{"SET", "EMPTY", "MISSING"})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, name := range []string{"EMPTY", "MISSING"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %s", err, name)
		}
	}
}

func TestEnvSliceSorted(t *testing.T) {
	got := EnvSlice(map[string]string{"B": "2", "A": "1"})
	if !reflect.DeepEqual(got, []string{"A=1", "B=2"}) {
		t.Errorf("got %v", got)
	}
}
