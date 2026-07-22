package deploy

import (
	"strings"
	"testing"
)

func TestRenderUnit(t *testing.T) {
	unit := RenderUnit("myapp", "/opt/myapp")
	for _, want := range []string{
		"ExecStart=/usr/local/bin/s99deploy run /opt/myapp",
		"User=myapp",
		"EnvironmentFile=/opt/myapp/.env",
		"WorkingDirectory=/opt/myapp/app",
		"ReadWritePaths=/opt/myapp",
		"WantedBy=multi-user.target",
	} {
		if !strings.Contains(unit, want) {
			t.Errorf("unit missing %q", want)
		}
	}
	if strings.Contains(unit, "__") {
		t.Errorf("unfilled placeholder in unit:\n%s", unit)
	}
}
