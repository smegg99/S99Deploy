// deploy/unit.go

package deploy

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed unit.service.tmpl
var unitSource string

var unitTemplate = template.Must(template.New("unit").Parse(unitSource))

// RenderUnit fills the embedded unit template. The service user is always
// named after the app, so name covers both.
func RenderUnit(name, root string) string {
	var out strings.Builder
	// The template is embedded and parsed at init, so execution cannot fail.
	_ = unitTemplate.Execute(&out, struct{ Name, Root string }{name, root})
	return out.String()
}
