// contract.go — omni-search (⌘K command palette) label + route contract types
// for the hybra cross-cutting search surface (Plan 20260710-omni-search, P2).
//
// The omni-search surface owns NO proto entity. It is a composition of existing
// read surfaces: one query fans out across the wave-1 entity categories and the
// results are rendered split by category, each row an href to the entity's
// tier-skinned detail page. The espyna service use case
// (uc.Service.OmniSearch.SearchEntities) does the gated, workspace-scoped query;
// this view layer owns only the palette chrome + result presentation.
//
// These types live in the `model` LEAF package (not the parent `omnisearch`
// package) so both the parent module and its action sub-package reference them
// without an import cycle — the same split the conversation surface uses.
package model

// ── Category presentation contract ───────────────────────────────────────────

// CategoryOrder is the wave-1 category presentation order. It is the hybra-side
// half of the registry-agreement contract: it MUST equal, key-for-key and in
// order, the espyna compile-time registry
// (packages/espyna-golang/internal/application/usecases/service/omnisearch/registry.go
// — wave1Categories). The registry-agreement test in this package pins that.
//
// Each key is ALSO the permission entity code the results handler checks
// (perms.Can("<key>", "list")) for the L2 fail-closed category filter, and the
// lookup key into OmniSearchLabels.Categories (tier heading) and
// DetailRoutePatterns (detail href pattern).
var CategoryOrder = []string{
	"client",
	"subscription",
	"subscription_group",
	"plan",
	"price_schedule",
	"product",
	"staff",
}

const (
	// DefaultMinChars is the minimum query length the palette searches on. Below
	// it the dialog shows the "type at least N characters" hint and (server-side)
	// returns the below-min state without calling the use case. Matches the
	// espyna use-case minQueryLen and the client debounce contract.
	DefaultMinChars = 2
	// DefaultLimitPerCategory is the per-category result cap the results handler
	// requests. The espyna use case clamps it server-side (cap 10); 5 is the
	// plan default.
	DefaultLimitPerCategory = 5
)

// DefaultDetailRoutePatterns returns the generic-tier (no-override) detail route
// patterns keyed by category key. The app supplies tier-skinned overrides at
// composition time (e.g. education renders /students/detail/{id} for the client
// category); these generic patterns are the standalone fallback so the module
// renders working links even before the app wires overrides.
//
// Patterns are UNPREFIXED (no /w/{slug}). The results handler prepends the
// session-derived workspace slug before substituting {id} — the codex blocker:
// /action/* is pass-through in the route rewriter, so hrefs rendered from a
// partial do NOT auto-prefix and must be built prefixed in Go.
func DefaultDetailRoutePatterns() map[string]string {
	return map[string]string{
		"client":             "/clients/detail/{id}",
		"subscription":       "/subscriptions/detail/{id}",
		"subscription_group": "/subscription-groups/detail/{id}",
		"plan":               "/plans/detail/{id}",
		"price_schedule":     "/price-schedules/detail/{id}",
		"product":            "/products/detail/{id}",
		// staff (wave-2) has NO detail page; a staff row's result id is its
		// workspace_user.id (resolved adapter-side), so the category links to the
		// generic workspace_user detail page. The app supplies the tier-resolved
		// override (workspace_user.detail) at composition time.
		"staff": "/workspace-users/detail/{id}",
	}
}

// ── Routes ───────────────────────────────────────────────────────────────────

// Omni-search route constants. Both are verb-first flat GET action routes under
// /action/ (read-only HTMX partials) — no {id} siblings, so the Go 1.22 ServeMux
// cannot conflict them. They are DELIBERATELY under /action/* (pass-through in
// the workspace route rewriter); the results handler compensates by building
// workspace-prefixed hrefs in Go.
const (
	OmniSearchDialogURL  = "/action/omni-search/dialog"
	OmniSearchResultsURL = "/action/omni-search/results"
)

// OmniSearchRoutes holds the palette's two GET action URLs.
type OmniSearchRoutes struct {
	DialogURL  string `json:"dialog_url"`  // /action/omni-search/dialog
	ResultsURL string `json:"results_url"` // /action/omni-search/results
}

// DefaultOmniSearchRoutes returns OmniSearchRoutes from the package constants.
func DefaultOmniSearchRoutes() OmniSearchRoutes {
	return OmniSearchRoutes{
		DialogURL:  OmniSearchDialogURL,
		ResultsURL: OmniSearchResultsURL,
	}
}

// RouteMap returns the dot-notation route keys → paths for the route-map
// descriptor (keys omni_search.dialog / omni_search.results).
func (r OmniSearchRoutes) RouteMap() map[string]string {
	return map[string]string{
		"omni_search.dialog":  r.DialogURL,
		"omni_search.results": r.ResultsURL,
	}
}

// ── Labels ───────────────────────────────────────────────────────────────────

// OmniSearchLabels is the palette chrome label struct. Loaded from
// translations/en/{tier}/omni_search.json (root key "omniSearch") via the app's
// LoadPathIfExists label flow. Every field is nil-safe: DefaultOmniSearchLabels
// pre-populates generic English so a missing JSON file does not surface empty
// strings — and the Go zero-value defaults use proto-generic wording ("Clients",
// never "Students"), tier vocabulary enters ONLY via lyngua.
//
// Hint carries the "{min}" placeholder and EmptyMessage the "{query}"
// placeholder; the handlers substitute them before render (html/template escapes
// the substituted query, so it is XSS-safe).
type OmniSearchLabels struct {
	Title        string            `json:"title"`
	Placeholder  string            `json:"placeholder"`
	Hint         string            `json:"hint"`
	ShortcutHint string            `json:"shortcut_hint"`
	Loading      string            `json:"loading"`
	EmptyTitle   string            `json:"empty_title"`
	EmptyMessage string            `json:"empty_message"`
	Error        string            `json:"error"`
	Close        string            `json:"close"`
	ViewAll      string            `json:"view_all"`
	NavHint      string            `json:"nav_hint"`
	Categories   map[string]string `json:"categories"` // keyed by generic category key
}

// DefaultOmniSearchLabels returns the generic-tier English defaults. The
// categories map authors all keys now (wave-1 six + wave-2 staff + the
// forward-looking job/job_template) so later waves are JSON-only; only
// CategoryOrder gates which sections currently render.
func DefaultOmniSearchLabels() OmniSearchLabels {
	return OmniSearchLabels{
		Title:        "Search",
		Placeholder:  "Search anything…",
		Hint:         "Type at least {min} characters",
		ShortcutHint: "⌘K / Alt+K",
		Loading:      "Searching…",
		EmptyTitle:   "No results",
		EmptyMessage: "Nothing matched \"{query}\".",
		Error:        "Search is unavailable right now. Try again.",
		Close:        "Close",
		ViewAll:      "View all",
		NavHint:      "↑↓ to navigate · Enter to open · Esc to close",
		Categories: map[string]string{
			"client":             "Clients",
			"subscription":       "Subscriptions",
			"subscription_group": "Groups",
			"plan":               "Plans",
			"price_schedule":     "Price Schedules",
			"product":            "Products",
			"staff":              "Staff",
			"job":                "Jobs",
			"job_template":       "Templates",
		},
	}
}

// ── View-model payloads ──────────────────────────────────────────────────────

// DialogData is the omni-search-dialog partial payload (the palette shell).
type DialogData struct {
	Labels   OmniSearchLabels
	Routes   OmniSearchRoutes
	MinChars int
	// Hint is Labels.Hint with {min} resolved.
	Hint string
}

// ResultRow is one rendered result. Href is the pre-built, workspace-prefixed
// detail URL (empty ⇒ the template renders a non-navigating row, e.g. when the
// category has no detail pattern wired).
type ResultRow struct {
	ID       string
	Label    string
	Sublabel string
	Href     string
}

// CategorySection groups one category's rendered rows under its tier heading.
type CategorySection struct {
	Key     string
	Heading string
	Rows    []ResultRow
}

// ResultsData is the omni-search-results-partial payload. Exactly one of the
// state flags drives the rendered branch: IsError → error chrome; BelowMin →
// the min-chars hint; !HasResults → the empty state; otherwise Sections.
type ResultsData struct {
	Labels   OmniSearchLabels
	Query    string
	Sections []CategorySection
	MinChars int

	HasResults bool
	BelowMin   bool
	IsError    bool

	// Hint is Labels.Hint with {min} resolved (BelowMin state).
	Hint string
	// EmptyMessage is Labels.EmptyMessage with {query} resolved (empty state).
	EmptyMessage string
}
