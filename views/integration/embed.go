package integration

import "embed"

//go:embed templates/*.html
var TemplatesFS embed.FS
