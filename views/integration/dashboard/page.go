// Package dashboard implements the read-only Integration live dashboard view
// (Phase 7 — Pyeza dashboard block + per-app live dashboards plan).
//
// The integration sidebar app is a *composition surface* — it owns no proto
// entity in cyta or centymo, and there is no dedicated `Integration` table.
// Aggregates today live per-provider (payment, email, scheduler, tabular) so
// the dashboard pulls from a typed Response shape that the use case fills in
// from whatever providers are wired. When a provider's stats hook is missing
// the use case returns dummy / zero values — the view degrades gracefully.
package dashboard

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"strings"

	pyeza "github.com/erniealice/pyeza-golang"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"

	hybra "github.com/erniealice/hybra-golang"
)

// ProviderRow is a typed row for the "By provider" widget. The use case
// projects per-provider stats into this shape so the view can render without
// proto-coupling.
type ProviderRow struct {
	ID         string
	Name       string
	Status     string // "active" | "error" | "disconnected"
	LastSync   string // human-readable timestamp ("2 minutes ago", "Yesterday")
	EventsLast7d int
}

// ErrorEntry is a typed row for the "Recent errors" list widget.
type ErrorEntry struct {
	ID         string
	Provider   string
	Message    string
	OccurredAt string
}

// Request mirrors the espyna use-case request.
type Request struct {
	WorkspaceID string
}

// Response mirrors the espyna use-case response. The view consumes only this
// shape and does not import any provider-specific protobuf types.
type Response struct {
	TotalIntegrations  int64
	ActiveIntegrations int64
	ErrorsLast24h      int64
	Disconnected       int64

	// 7-day sync events trend.
	TrendLabels []string
	TrendValues []float64

	// Top providers ordered by event volume / status priority.
	Providers []ProviderRow

	// Most recent N error entries across all providers.
	RecentErrors []ErrorEntry
}

// Deps holds view dependencies. All callbacks are nil-safe — the view uses
// dummy data when GetDashboardPageData is nil (typical when the orchestrator
// has not yet wired any provider stats hooks).
type Deps struct {
	Routes               hybra.IntegrationRoutes
	Labels               hybra.IntegrationLabels
	CommonLabels         pyeza.CommonLabels
	GetDashboardPageData func(ctx context.Context, req *Request) (*Response, error)
}

// PageData is the dashboard template payload.
type PageData struct {
	types.PageData
	ContentTemplate string
	Dashboard       types.DashboardData
}

// NewView creates the integration dashboard view.
func NewView(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		l := deps.Labels.Dashboard

		var resp *Response
		if deps.GetDashboardPageData != nil {
			r, err := deps.GetDashboardPageData(ctx, &Request{WorkspaceID: ""})
			if err == nil && r != nil {
				resp = r
			}
		}
		if resp == nil {
			resp = &Response{}
		}

		// Sync-events trend (7 days). Default to flat-zero when no data.
		labels := resp.TrendLabels
		values := resp.TrendValues
		if len(labels) == 0 {
			labels = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
			values = []float64{0, 0, 0, 0, 0, 0, 0}
		}
		trend := &types.ChartData{
			Labels: labels,
			Series: []types.ChartSeries{{
				Name:   l.SyncEventsTrend,
				Values: values,
				Color:  "teal",
			}},
			YAxis: l.AxisEvents,
		}
		trend.AutoScale()

		// Recent errors → activity list.
		errors := buildRecentErrorsList(resp.RecentErrors)

		dash := types.DashboardData{
			Title:    l.Title,
			Icon:     "icon-zap",
			Subtitle: l.Subtitle,
			QuickActions: []types.QuickAction{
				{Icon: "icon-plus", Label: l.QuickConnect, Href: deps.Routes.ConnectionsURL, Variant: "primary", TestID: "integration-action-connect"},
				{Icon: "icon-refresh-cw", Label: l.QuickTestSync, Href: deps.Routes.ConnectionsURL, TestID: "integration-action-test-sync"},
				{Icon: "icon-list", Label: l.QuickViewLogs, Href: deps.Routes.ConnectionsURL, TestID: "integration-action-view-logs"},
				{Icon: "icon-settings", Label: l.QuickConfigureWebhook, Href: deps.Routes.ConnectionsURL, TestID: "integration-action-configure-webhook"},
			},
			Stats: []types.StatCardData{
				{Icon: "icon-link", Value: fmt.Sprintf("%d", resp.TotalIntegrations), Label: l.TotalIntegrations, Color: "navy", TestID: "integration-stat-total"},
				{Icon: "icon-zap", Value: fmt.Sprintf("%d", resp.ActiveIntegrations), Label: l.ActiveIntegrations, Color: "sage", TestID: "integration-stat-active"},
				{Icon: "icon-alert-triangle", Value: fmt.Sprintf("%d", resp.ErrorsLast24h), Label: l.ErrorsLast24h, Color: "terracotta", TestID: "integration-stat-errors"},
				{Icon: "icon-x-circle", Value: fmt.Sprintf("%d", resp.Disconnected), Label: l.Disconnected, Color: "amber", TestID: "integration-stat-disconnected"},
			},
			Widgets: []types.DashboardWidget{
				{
					ID: "sync-trend", Title: l.SyncEventsTrend,
					Type: "chart", ChartKind: "line",
					ChartData: trend, Span: 2,
				},
				{
					ID:    "by-provider",
					Title: l.ByProvider,
					Type:  "custom", Span: 2,
					Custom: buildProviderTableHTML(resp.Providers, l),
					EmptyState: &types.EmptyStateData{
						Icon: "icon-link", Title: l.ByProvider, Desc: l.NoProviders,
					},
				},
				{
					ID: "recent-errors", Title: l.RecentErrors, Type: "list", Span: 1,
					HeaderActions: []types.QuickAction{
						{Label: l.ViewAll, Href: deps.Routes.ConnectionsURL},
					},
					ListItems: errors,
					EmptyState: &types.EmptyStateData{
						Icon: "icon-shield", Title: l.RecentErrors, Desc: l.NoRecentErrors,
					},
				},
			},
		}

		pageData := &PageData{
			PageData: types.PageData{
				CacheVersion:   viewCtx.CacheVersion,
				Title:          l.Title,
				CurrentPath:    viewCtx.CurrentPath,
				ActiveNav:      "integration",
				ActiveSubNav:   "dashboard",
				HeaderTitle:    l.Title,
				HeaderSubtitle: l.Subtitle,
				HeaderIcon:     "icon-zap",
				CommonLabels:   deps.CommonLabels,
			},
			ContentTemplate: "integration-dashboard-content",
			Dashboard:       dash,
		}
		return view.OK("integration-dashboard", pageData)
	})
}

func buildRecentErrorsList(errors []ErrorEntry) []types.ActivityItem {
	if len(errors) == 0 {
		return nil
	}
	items := make([]types.ActivityItem, 0, len(errors))
	for i, e := range errors {
		items = append(items, types.ActivityItem{
			IconName:    "icon-alert-triangle",
			IconVariant: "integration",
			Title:       e.Provider,
			Description: e.Message,
			Time:        e.OccurredAt,
			TestID:      fmt.Sprintf("integration-list-item-%d", i),
		})
	}
	return items
}

func buildProviderTableHTML(rows []ProviderRow, l hybra.IntegrationDashboardLabels) template.HTML {
	if len(rows) == 0 {
		return ""
	}
	var b bytes.Buffer
	b.WriteString(`<table class="dashboard-mini-table"><thead><tr>`)
	b.WriteString(`<th>` + template.HTMLEscapeString(l.ProviderColumn) + `</th>`)
	b.WriteString(`<th>` + template.HTMLEscapeString(l.StatusColumn) + `</th>`)
	b.WriteString(`<th class="numeric">` + template.HTMLEscapeString(l.AxisEvents) + `</th>`)
	b.WriteString(`<th>` + template.HTMLEscapeString(l.LastSyncColumn) + `</th>`)
	b.WriteString(`</tr></thead><tbody>`)
	for _, r := range rows {
		b.WriteString(`<tr data-testid="integration-table-row-`)
		b.WriteString(template.HTMLEscapeString(r.ID))
		b.WriteString(`">`)
		b.WriteString(`<td>` + template.HTMLEscapeString(r.Name) + `</td>`)
		b.WriteString(`<td><span class="badge badge-` + template.HTMLEscapeString(strings.ToLower(r.Status)) + `">` + template.HTMLEscapeString(r.Status) + `</span></td>`)
		b.WriteString(fmt.Sprintf(`<td class="numeric">%d</td>`, r.EventsLast7d))
		b.WriteString(`<td>` + template.HTMLEscapeString(r.LastSync) + `</td>`)
		b.WriteString(`</tr>`)
	}
	b.WriteString(`</tbody></table>`)
	return template.HTML(b.String()) //nolint:gosec
}
