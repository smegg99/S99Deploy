package deploy

import (
	"strings"
	"testing"
)

func TestResolveArgv0(t *testing.T) {
	if got, _ := resolveArgv0("/opt/x/app", "/usr/bin/env"); got != "/usr/bin/env" {
		t.Errorf("absolute: %q", got)
	}
	if got, _ := resolveArgv0("/opt/x/app", "bin/server"); got != "/opt/x/app/bin/server" {
		t.Errorf("relative: %q", got)
	}
	got, err := resolveArgv0("/opt/x/app", "sh")
	if err != nil || !strings.HasSuffix(got, "/sh") {
		t.Errorf("bare name via PATH: %q, %v", got, err)
	}
}
