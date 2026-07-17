package omnisearch

import "embed"

// TemplatesFS holds the omni-search dialog + results partial templates. The app
// registers this FS with its renderer (both apps, per the conversation
// precedent) so the ViewAdapter can render the palette partials.
//
//go:embed templates/*.html
var TemplatesFS embed.FS
