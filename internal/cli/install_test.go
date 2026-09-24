// internal/cli/install_test.go

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/smegg99/s99deploy/internal/deploy"
)

// The refusal is only useful if the reader can act on it, in the language the
// rest of the tool speaks.
func TestInstallRefusesAnUnreachableSelfPathInTheReadersLanguage(t *testing.T) {
	const self = "/home/dev/S99Deploy/bin/depl"
	for _, c := range []struct {
		lang string
		want []string
	}{
		{lang: "en", want: []string{"ProtectHome=true", self, "/usr/local/bin", "just install"}},
		{lang: "pl", want: []string{"ProtectHome=true", self, "zainstaluj najpierw depl", "/usr/local/bin"}},
	} {
		t.Run(c.lang, func(t *testing.T) {
			opts, _, stderr := testOptionsOn(t, "", func(cfg *deploy.Config) {
				cfg.SelfPath = self
			})

			code := run(context.Background(), opts,
				[]string{"--lang=" + c.lang, "install", "https://example.com/myapp.git"})

			if code != 1 {
				t.Errorf("code = %d, want 1", code)
			}
			for _, want := range c.want {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr is missing %q:\n%s", want, stderr)
				}
			}
		})
	}
}
