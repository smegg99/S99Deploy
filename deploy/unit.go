// deploy/unit.go

package deploy

import (
	_ "embed"
	"strings"
)

//go:embed unit.service.tmpl
var unitTemplate string

// RenderUnit fills the embedded unit template. The service user is always
// named after the app, so name covers both.
func RenderUnit(name, root string) string {
	unit := strings.ReplaceAll(unitTemplate, "__NAME__", name)
	return strings.ReplaceAll(unit, "__ROOT__", root)
}
