package hybra

// Three-level routing system for hybra integration views:
//
// Level 1: Generic defaults from Go consts (routes.go).
// Level 2: Industry-specific overrides via JSON (loaded by consumer apps).
// Level 3: App-specific overrides via Go field assignment (optional).

// IntegrationRoutes holds all route paths for the integration composition surface.
// The orchestrator constructs this with DefaultIntegrationRoutes() and threads it
// through the integration view module.
type IntegrationRoutes struct {
	ActiveNav string `json:"active_nav"`

	// DashboardURL is the integration live dashboard (Phase 7).
	DashboardURL string `json:"dashboard_url"`

	// ConnectionsURL is the legacy /app/integrations connections list. Kept
	// here as a Routes field so the dashboard's quick-actions can link to it
	// without hardcoding the URL.
	ConnectionsURL string `json:"connections_url"`

	// SchedulingURL is the calendar-sync sub-app surface.
	SchedulingURL string `json:"scheduling_url"`

	// FulfillmentURL is the delivery-partner sub-app surface.
	FulfillmentURL string `json:"fulfillment_url"`
}

// DefaultIntegrationRoutes returns IntegrationRoutes populated from package-level
// constants in routes.go. Sidebar URLs that aren't promoted to constants here
// (connections / scheduling / fulfillment) match the existing service-admin
// sidebar.go hardcoded values.
func DefaultIntegrationRoutes() IntegrationRoutes {
	return IntegrationRoutes{
		ActiveNav:      "integration",
		DashboardURL:   IntegrationDashboardURL,
		ConnectionsURL: "/app/integrations",
		SchedulingURL:  "/app/integrations/scheduling",
		FulfillmentURL: "/app/integrations/fulfillment",
	}
}

// RouteMap returns a map[string]string for template URL resolution.
func (r IntegrationRoutes) RouteMap() map[string]string {
	return map[string]string{
		"integration.dashboard":   r.DashboardURL,
		"integration.connections": r.ConnectionsURL,
		"integration.scheduling":  r.SchedulingURL,
		"integration.fulfillment": r.FulfillmentURL,
	}
}
