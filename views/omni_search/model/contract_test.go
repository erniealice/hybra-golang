package model

import (
	"reflect"
	"testing"
)

// canonicalWave1Keys is the espyna compile-time registry's wave-1 category set,
// in registry (presentation) order. SOURCE OF TRUTH:
// packages/espyna-golang/internal/application/usecases/service/omnisearch/registry.go
// (wave1Categories). That package is espyna-internal and cannot be imported
// across the module boundary, so this frozen copy IS the hybra half of the
// registry-agreement contract: the presentation layer's CategoryOrder must equal
// the use-case layer's category keys, key-for-key and in order, or the palette
// would render a section the use case never gates (or omit one it does).
var canonicalWave1Keys = []string{
	"client",
	"subscription",
	"subscription_group",
	"plan",
	"price_schedule",
	"product",
	"staff",
}

// TestCategoryOrderMatchesRegistry pins the espyna↔hybra registry agreement.
func TestCategoryOrderMatchesRegistry(t *testing.T) {
	if !reflect.DeepEqual(CategoryOrder, canonicalWave1Keys) {
		t.Fatalf("CategoryOrder drifted from the espyna omni-search registry\n got=%v\nwant=%v\n(sync with espyna .../service/omnisearch/registry.go wave1Categories)",
			CategoryOrder, canonicalWave1Keys)
	}
}

// TestEveryCategoryHasHeadingAndPattern guards internal presentation drift: every
// rendered category must resolve to a tier heading (Labels.Categories) and a
// detail route pattern (DefaultDetailRoutePatterns), else a section would render
// with a bare key heading or dead (empty-href) rows.
func TestEveryCategoryHasHeadingAndPattern(t *testing.T) {
	labels := DefaultOmniSearchLabels()
	patterns := DefaultDetailRoutePatterns()

	for _, key := range CategoryOrder {
		if labels.Categories[key] == "" {
			t.Errorf("category %q has no default heading in DefaultOmniSearchLabels().Categories", key)
		}
		if patterns[key] == "" {
			t.Errorf("category %q has no default detail pattern in DefaultDetailRoutePatterns()", key)
		}
	}
}

// TestRouteMapKeys pins the route-map descriptor keys the app loads.
func TestRouteMapKeys(t *testing.T) {
	rm := DefaultOmniSearchRoutes().RouteMap()
	for _, key := range []string{"omni_search.dialog", "omni_search.results"} {
		if rm[key] == "" {
			t.Errorf("route map missing key %q", key)
		}
	}
}
