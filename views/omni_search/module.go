// Package omnisearch is the cross-cutting view module for the omni-search
// (⌘K "search anything" command palette) surface — Plan 20260710-omni-search, P2.
//
// It owns no proto entity. Two GET action partials compose it: a dialog shell
// (lazy-loaded palette chrome) and a results partial (the gated cross-entity
// query render). The espyna service use case
// (uc.Service.OmniSearch.SearchEntities) does the workspace-scoped, per-category
// gated query; this module owns only the palette chrome + result presentation,
// building each result's tier-skinned, workspace-prefixed detail href in Go.
//
// Nil-safe: with no use case wired the results view renders an empty palette
// (never a 500) — the fail-closed "search unavailable" degrade.
package omnisearch

import (
	"context"

	"github.com/erniealice/pyeza-golang/view"

	omnisearchpb "github.com/erniealice/esqyma/pkg/schema/v1/service/omni_search"

	omniaction "github.com/erniealice/hybra-golang/views/omni_search/action"
	omnimodel "github.com/erniealice/hybra-golang/views/omni_search/model"
)

// Routes / Labels aliases so the module's public API reads naturally while the
// canonical contract types stay in the model leaf package (single source of
// truth). Callers build ModuleDeps with omnisearch.Routes{} / omnisearch.Labels{}
// or the model.Default* constructors.
type (
	Routes = omnimodel.OmniSearchRoutes
	Labels = omnimodel.OmniSearchLabels
)

// ModuleDeps holds all dependencies for the omni-search view module.
type ModuleDeps struct {
	Routes Routes
	Labels Labels

	// MinChars overrides the default minimum query length when > 0.
	MinChars int

	// SearchEntities is the espyna omni-search use case as a typed closure
	// (uc.Service.OmniSearch.SearchEntities.Execute). Nil-safe: empty palette.
	SearchEntities func(ctx context.Context, req *omnisearchpb.OmniSearchRequest) (*omnisearchpb.OmniSearchResponse, error)

	// DetailRoutePatterns maps category key → tier-skinned detail pattern
	// ("/students/detail/{id}"). Nil/empty ⇒ generic defaults.
	DetailRoutePatterns map[string]string

	// WorkspaceSlug resolves the session workspace slug from ctx for the
	// /w/{slug} href prefix. Nil-safe (⇒ no prefix).
	WorkspaceSlug func(ctx context.Context) string
}

// Module holds the constructed omni-search views.
type Module struct {
	Dialog  view.View
	Results view.View
	routes  omnimodel.OmniSearchRoutes
}

// NewModule constructs the omni-search view module from typed deps. Nil-safe:
// missing routes/labels fall back to package defaults; a missing use case yields
// an empty palette.
func NewModule(deps *ModuleDeps) *Module {
	if deps == nil {
		deps = &ModuleDeps{}
	}

	routes := deps.Routes
	if routes.DialogURL == "" && routes.ResultsURL == "" {
		routes = omnimodel.DefaultOmniSearchRoutes()
	}

	labels := deps.Labels
	if labels.Categories == nil {
		labels = omnimodel.DefaultOmniSearchLabels()
	}

	actionDeps := &omniaction.Deps{
		Routes:              routes,
		Labels:              labels,
		MinChars:            deps.MinChars,
		SearchEntities:      deps.SearchEntities,
		DetailRoutePatterns: deps.DetailRoutePatterns,
		WorkspaceSlug:       deps.WorkspaceSlug,
	}

	return &Module{
		Dialog:  omniaction.NewDialogAction(actionDeps),
		Results: omniaction.NewResultsAction(actionDeps),
		routes:  routes,
	}
}

// RegisterRoutes registers the two GET action routes.
func (m *Module) RegisterRoutes(r view.RouteRegistrar) {
	if m.Dialog != nil && m.routes.DialogURL != "" {
		r.GET(m.routes.DialogURL, m.Dialog)
	}
	if m.Results != nil && m.routes.ResultsURL != "" {
		r.GET(m.routes.ResultsURL, m.Results)
	}
}
