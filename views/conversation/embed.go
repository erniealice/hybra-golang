package conversation

import "embed"

// TemplatesFS embeds all conversation view templates. Registered with the
// pyeza HTML renderer by service-admin's container wiring (Phase 4).
//
//go:embed templates/*.html
var TemplatesFS embed.FS
