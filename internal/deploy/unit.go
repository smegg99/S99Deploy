// internal/deploy/unit.go

package deploy

import (
	_ "embed"
	"strings"
	"text/template"
)

//go:embed unit.service.tmpl
var unitSource string

// unitMarker is in every unit this tool renders, so it can tell its own from a foreign one.
const unitMarker = "(managed by s99deploy)"

var unitTemplate = template.Must(template.New("unit").Parse(unitSource))

// UnitParams is what the unit template needs.
type UnitParams struct{ Name, Root, SelfPath string }

// RenderUnit fills the embedded unit template.
func RenderUnit(u UnitParams) string {
	var out strings.Builder
	// The template is embedded and parsed at init, so execution cannot fail.
	_ = unitTemplate.Execute(&out, u)
	return out.String()
}
