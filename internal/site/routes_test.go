// internal/site/routes_test.go

package site

import (
	"net/http"
	"testing"
)

// Every route the server registers answers 200.
func TestEveryRouteAnswers(t *testing.T) {
	router, _ := serving(t, "ELFBYTES")

	for _, path := range []string{"/", "/install.sh", "/s99deploy", "/s99deploy.sha256"} {
		t.Run(path, func(t *testing.T) {
			if code := fetch(t, router, path).Code; code != http.StatusOK {
				t.Errorf("GET %s = %d, want 200", path, code)
			}
		})
	}
}
