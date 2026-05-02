// Package hybra — route constants for cross-cutting UI surfaces.
//
// The integration app is a composition surface (no first-class proto entity).
// Phase 7 of the pyeza-dashboard plan introduces /app/integrations/dashboard as
// the canonical mount-point for the integration overview view.
package hybra

// Default route constants for the integration composition surface.
// Consumer apps may override these via IntegrationRoutes JSON or Go field
// assignment when wiring the module.
const (
	// IntegrationDashboardURL is the canonical mount for the integration
	// live dashboard. Today the sidebar entry "{Key: integration, ... URL:
	// /app/integrations}" is hardcoded in service-admin/sidebar.go to land
	// on the legacy connections list. Phase 7 introduces this dashboard URL
	// as the new top-of-sidebar entry; the orchestrator follow-up will swap
	// the sidebar URL to point here.
	IntegrationDashboardURL = "/app/integrations/dashboard"
)
