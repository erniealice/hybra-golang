package action

import (
	"context"
	"strings"
	"unicode/utf8"

	omnisearchpb "github.com/erniealice/esqyma/pkg/schema/v1/service/omni_search"
	"github.com/erniealice/pyeza-golang/view"

	"github.com/erniealice/hybra-golang/views/omni_search/model"
)

// NewResultsAction returns the GET /action/omni-search/results?q=… handler. It
// renders the grouped result sections as an HTML partial. Authorization is
// layered:
//
//	L2 (here): the category key IS the permission entity code, so it filters the
//	    wave-1 categories by perms.Can("<key>", "list") and short-circuits to the
//	    empty state when the principal can list none — fail-closed (nil perms →
//	    Can returns false), so the view never even queries a denied category.
//	L4 (espyna use case): re-gates the passed subset via ActionGatekeeper, so a
//	    future caller that bypassed this filter still cannot widen the result set.
//
// Every branch returns 200 with no-store headers so HTMX swaps the partial in;
// a use-case error renders the error CHROME (never a 500), and a missing use
// case renders the empty palette (the fail-closed degrade).
func NewResultsAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		minChars := deps.minChars()
		query := strings.TrimSpace(queryParam(viewCtx.Request, "q"))

		data := &model.ResultsData{
			Labels:   deps.Labels,
			Query:    query,
			MinChars: minChars,
		}
		res := view.OK("omni-search-results-partial", data)
		res.Headers = noStoreHeaders()

		// Below the minimum → the hint, no query fired.
		if utf8.RuneCountInString(query) < minChars {
			data.BelowMin = true
			data.Hint = resolveHint(deps.Labels.Hint, minChars)
			return res
		}

		// L2 fail-closed per-category filter (category key == permission entity).
		perms := view.GetUserPermissions(ctx)
		permitted := make([]string, 0, len(model.CategoryOrder))
		for _, key := range model.CategoryOrder {
			if perms.Can(key, "list") {
				permitted = append(permitted, key)
			}
		}

		// No permitted category, or no use case wired → empty palette (never 500).
		if len(permitted) == 0 || deps.SearchEntities == nil {
			data.EmptyMessage = resolveEmpty(deps.Labels.EmptyMessage, query)
			return res
		}

		limit := int32(model.DefaultLimitPerCategory)
		resp, err := deps.SearchEntities(ctx, &omnisearchpb.OmniSearchRequest{
			Query:            query,
			LimitPerCategory: &limit,
			Categories:       permitted,
		})
		if err != nil || resp == nil {
			data.IsError = true
			return res
		}

		data.Sections = buildSections(ctx, deps, resp)
		data.HasResults = len(data.Sections) > 0
		if !data.HasResults {
			data.EmptyMessage = resolveEmpty(deps.Labels.EmptyMessage, query)
		}
		return res
	})
}

// buildSections projects the use-case response into rendered sections, in the
// fixed CategoryOrder, skipping categories with no rows. Each row's detail href
// is built workspace-prefixed in Go (the /action/* prefix blocker).
func buildSections(ctx context.Context, deps *Deps, resp *omnisearchpb.OmniSearchResponse) []model.CategorySection {
	byKey := make(map[string][]*omnisearchpb.OmniSearchResult, len(resp.GetCategories()))
	for _, c := range resp.GetCategories() {
		byKey[c.GetCategory()] = c.GetResults()
	}

	slug := deps.workspaceSlug(ctx)
	patterns := deps.detailPatterns()

	var sections []model.CategorySection
	for _, key := range model.CategoryOrder {
		rows := byKey[key]
		if len(rows) == 0 {
			continue
		}
		heading := deps.Labels.Categories[key]
		if heading == "" {
			heading = key
		}
		pattern := patterns[key]

		sec := model.CategorySection{Key: key, Heading: heading}
		for _, r := range rows {
			sec.Rows = append(sec.Rows, model.ResultRow{
				ID:       r.GetId(),
				Label:    r.GetLabel(),
				Sublabel: r.GetSublabel(),
				Href:     buildHref(pattern, slug, r.GetId()),
			})
		}
		sections = append(sections, sec)
	}
	return sections
}
