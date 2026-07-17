package omnisearch

import (
	"html/template"
	"testing"
)

// TestTemplatesParse guarantees the embedded palette templates parse and define
// the two named blocks the action handlers render (view.OK template names). A
// parse failure or a renamed define would otherwise only surface at request time
// as a blank partial.
func TestTemplatesParse(t *testing.T) {
	tmpl, err := template.New("omni-search").ParseFS(TemplatesFS, "templates/*.html")
	if err != nil {
		t.Fatalf("parsing omni-search templates: %v", err)
	}
	for _, name := range []string{"omni-search-dialog", "omni-search-results-partial"} {
		if tmpl.Lookup(name) == nil {
			t.Errorf("template %q not defined in TemplatesFS", name)
		}
	}
}
