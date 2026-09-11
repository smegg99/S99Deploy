// internal/deploy/unit_test.go

package deploy

import (
	"strings"
	"testing"
)

func TestRenderUnit(t *testing.T) {
	// Not the production path: a template that hardcoded it would pass.
	unit := RenderUnit(UnitParams{
		Name: "myapp", Root: "/opt/myapp", SelfPath: "/tmp/elsewhere/s99deploy",
	})

	for _, want := range []string{
		"ExecStart=/tmp/elsewhere/s99deploy run /opt/myapp",
		"User=myapp",
		"EnvironmentFile=/opt/myapp/.env",
		"WorkingDirectory=/opt/myapp/app",
		"ReadWritePaths=/opt/myapp/app",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit missing %q", want)
		}
	}
	if strings.Contains(unit, "{{") || strings.Contains(unit, "<no value>") {
		t.Errorf("unfilled placeholder in unit:\n%s", unit)
	}
}

// The resolver, journald, D-Bus and an S99AB work/daemon socket are all AF_UNIX.
func TestUnitAllowsUnixSockets(t *testing.T) {
	// Denying it bought nothing against a process that already has AF_INET.
	unit := RenderUnit(UnitParams{
		Name: "myapp", Root: "/opt/myapp", SelfPath: "/usr/local/bin/s99deploy",
	})

	const want = "RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6"
	var families string
	for _, line := range strings.Split(unit, "\n") {
		if strings.HasPrefix(line, "RestrictAddressFamilies=") {
			families = line
		}
	}
	if families != want {
		t.Errorf("unit does not carry %q:\n%s", want, unit)
	}
	for _, denied := range []string{"AF_PACKET", "AF_NETLINK"} {
		if strings.Contains(families, denied) {
			t.Errorf("%s is listed; the families this option exists to block must stay blocked", denied)
		}
	}
}

// A path the tool builds must not be able to hide a second directive.
func TestRenderUnitRefusesNothingItIsNotGiven(t *testing.T) {
	unit := RenderUnit(UnitParams{Name: "myapp", Root: "/opt/myapp", SelfPath: "/usr/local/bin/s99deploy"})

	if strings.Count(unit, "ExecStart=") != 1 {
		t.Errorf("want exactly one ExecStart:\n%s", unit)
	}
}
