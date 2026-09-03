// version_test.go

package s99deploy

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestDeclaredComesFromTheVersionFile(t *testing.T) {
	body, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(body))

	if want == "" {
		t.Fatal("VERSION is empty")
	}
	if got := Declared(); got != want {
		t.Fatalf("Declared() = %q, want %q", got, want)
	}
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(want) {
		t.Fatalf("VERSION = %q, want MAJOR.MINOR.PATCH", want)
	}
}

// The three shapes a version can print.
func TestVersionShapes(t *testing.T) {
	// format covers revisions absent from test binary metadata.
	for _, c := range []struct {
		rev   string
		dirty bool
		want  string
	}{
		{rev: "", dirty: false, want: "1.0.0"},
		{rev: "a22e8d3", dirty: false, want: "1.0.0 (a22e8d3)"},
		{rev: "a22e8d3", dirty: true, want: "1.0.0 (a22e8d3, dirty)"},
	} {
		if got := format("1.0.0", c.rev, c.dirty); got != c.want {
			t.Errorf("format(1.0.0, %q, %v) = %q, want %q", c.rev, c.dirty, got, c.want)
		}
	}
}

// A tagged commit whose VERSION disagrees ships a module that misreports itself.
func TestTagAgreesWithVersionFile(t *testing.T) {
	// A tag is what "go get ...@v1.0.0" resolves.
	tag, ok := exactTag()
	if !ok {
		t.Skip("HEAD carries no tag")
	}

	if want := "v" + Declared(); tag != want {
		t.Fatalf("HEAD is tagged %s but VERSION says %s; one of them is wrong",
			tag, Declared())
	}
}

// exactTag returns the tag on HEAD, absent outside a git worktree.
func exactTag() (string, bool) {
	git, err := exec.LookPath("git")
	if err != nil {
		return "", false
	}
	if err := exec.Command(git, "rev-parse", "--git-dir").Run(); err != nil {
		return "", false
	}

	out, err := exec.Command(git, "describe", "--tags", "--exact-match").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}
