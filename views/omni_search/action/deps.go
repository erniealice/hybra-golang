// Package action holds the two GET HTMX action handlers for the omni-search
// palette: the dialog shell (lazy-loaded chrome) and the results partial (the
// gated cross-entity query render).
//
// The espyna use case arrives as a typed closure — no consumer/* imports (the
// block-decouple invariant). The workspace slug arrives as a nil-safe closure so
// the results handler can build workspace-prefixed detail hrefs WITHOUT importing
// espyna/consumer to read the session workspace.
package action

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	omnisearchpb "github.com/erniealice/esqyma/pkg/schema/v1/service/omni_search"

	"github.com/erniealice/hybra-golang/views/omni_search/model"
)

// Deps holds dependencies shared by both omni-search action handlers.
type Deps struct {
	Routes model.OmniSearchRoutes
	Labels model.OmniSearchLabels

	// MinChars overrides model.DefaultMinChars when > 0.
	MinChars int

	// SearchEntities is the espyna omni-search use case
	// (uc.Service.OmniSearch.SearchEntities.Execute) as a typed closure. Nil-safe:
	// when nil the results handler renders an empty palette (never a 500) — the
	// fail-closed "no use case wired" degrade.
	SearchEntities func(ctx context.Context, req *omnisearchpb.OmniSearchRequest) (*omnisearchpb.OmniSearchResponse, error)

	// DetailRoutePatterns maps category key → the tier-skinned detail route
	// pattern (with a "{id}" placeholder). Nil/empty ⇒ model.DefaultDetailRoutePatterns.
	// Supplied by the app from its Layer-3 route map so education renders
	// /students/detail/{id} where service renders /clients/detail/{id}.
	DetailRoutePatterns map[string]string

	// WorkspaceSlug resolves the SESSION-derived workspace slug from ctx. Nil-safe
	// (returns "" ⇒ no /w/{slug} prefix, matching the boot-time route map). The
	// app supplies a trusted resolver (session workspace identity, never the
	// client HX-Current-URL header) — the codex prefix blocker.
	WorkspaceSlug func(ctx context.Context) string
}

// minChars returns the effective minimum query length.
func (d *Deps) minChars() int {
	if d.MinChars > 0 {
		return d.MinChars
	}
	return model.DefaultMinChars
}

// detailPatterns returns the effective category→pattern map.
func (d *Deps) detailPatterns() map[string]string {
	if len(d.DetailRoutePatterns) == 0 {
		return model.DefaultDetailRoutePatterns()
	}
	return d.DetailRoutePatterns
}

// workspaceSlug is the nil-safe session slug accessor.
func (d *Deps) workspaceSlug(ctx context.Context) string {
	if d.WorkspaceSlug == nil {
		return ""
	}
	return d.WorkspaceSlug(ctx)
}

// noStoreHeaders is the response hygiene contract for both GET partials
// (codex finding #3): a GET query lands client names / enrollment labels in the
// body and the raw query in the URL, so both are kept out of caches/proxies.
func noStoreHeaders() map[string]string {
	return map[string]string{
		"Cache-Control": "no-store, private",
		"Vary":          "HX-Request",
		"X-Robots-Tag":  "noindex",
	}
}

// resolveHint substitutes {min} in the hint label.
func resolveHint(hint string, minChars int) string {
	return strings.ReplaceAll(hint, "{min}", strconv.Itoa(minChars))
}

// resolveEmpty substitutes {query} in the empty-state label. html/template
// escapes the result at render time, so an attacker-controlled query cannot
// inject markup.
func resolveEmpty(msg, query string) string {
	return strings.ReplaceAll(msg, "{query}", query)
}

// buildHref builds the workspace-prefixed, id-filled detail href. It returns ""
// (a non-navigating row) when the category has no pattern or the row has no id.
// The slug is prepended BEFORE {id} substitution so a slug that (defensively)
// contained "{id}" could not be reinterpreted. The id is path-escaped.
func buildHref(pattern, slug, id string) string {
	if pattern == "" || id == "" {
		return ""
	}
	p := pattern
	if slug != "" {
		p = "/w/" + slug + p
	}
	return strings.ReplaceAll(p, "{id}", url.PathEscape(id))
}

// queryParam reads a single query-string value.
func queryParam(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	return r.URL.Query().Get(key)
}
