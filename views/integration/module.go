// Package integration is the cross-cutting view module for the integration
// composition surface (Phase 7 — Pyeza dashboard block + per-app live
// dashboards plan).
//
// Integration owns no proto entity. The dashboard pulls aggregates via a
// nil-safe callback that the orchestrator wires once provider stats hooks
// are available; until then the dashboard renders dummy / zero values.
package integration

import (
	"context"

	pyeza "github.com/erniealice/pyeza-golang"
	"github.com/erniealice/pyeza-golang/view"

	hybra "github.com/erniealice/hybra-golang"
	dashboardview "github.com/erniealice/hybra-golang/views/integration/dashboard"
)

// ModuleDeps holds all dependencies for the integration module.
type ModuleDeps struct {
	Routes       hybra.IntegrationRoutes
	Labels       hybra.IntegrationLabels
	CommonLabels pyeza.CommonLabels

	// nil-safe: dashboard renders dummy/zero values when not provided.
	GetIntegrationDashboardPageData func(ctx context.Context, req *dashboardview.Request) (*dashboardview.Response, error)
}

// Module holds all constructed integration views.
type Module struct {
	Dashboard view.View
	routes    hybra.IntegrationRoutes
}

// NewModule constructs the integration view module. Nil-safe: views render
// empty / dummy state when callbacks are missing.
func NewModule(deps *ModuleDeps) *Module {
	if deps == nil {
		deps = &ModuleDeps{}
	}

	routes := deps.Routes
	if routes.DashboardURL == "" {
		routes = hybra.DefaultIntegrationRoutes()
	}

	dashDeps := &dashboardview.Deps{
		Routes:               routes,
		Labels:               deps.Labels,
		CommonLabels:         deps.CommonLabels,
		GetDashboardPageData: deps.GetIntegrationDashboardPageData,
	}

	return &Module{
		Dashboard: dashboardview.NewView(dashDeps),
		routes:    routes,
	}
}

// RegisterRoutes registers all integration GET routes with the given
// route registrar.
func (m *Module) RegisterRoutes(r view.RouteRegistrar) {
	if m.Dashboard != nil && m.routes.DashboardURL != "" {
		r.GET(m.routes.DashboardURL, m.Dashboard)
	}
}
