// Package hybra holds cross-cutting view + label types for non-domain UI surfaces
// (integration composition surface, generic webhook + audit views, etc.).
//
// The integration sidebar app is a *composition surface*: it owns no proto entity
// in cyta or centymo. Today its sidebar entry mounts at /app/integrations as a
// hardcoded URL. Phase 7 of the pyeza-dashboard-block plan adds a real dashboard
// at /app/integrations/dashboard, owned by this package.
package hybra

// ---------------------------------------------------------------------------
// Integration labels — owns the integration composition surface (sidebar app).
// ---------------------------------------------------------------------------

// IntegrationLabels is the top-level label container for the integration app.
// Loaded from lyngua translation files (translations/{locale}/{tier}/integration.json).
type IntegrationLabels struct {
	Dashboard IntegrationDashboardLabels `json:"dashboard"`
}

// IntegrationDashboardLabels holds translatable strings for the Integration
// live dashboard (Phase 7 — Pyeza dashboard block + per-app live dashboards plan).
type IntegrationDashboardLabels struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`

	// Stats
	TotalIntegrations  string `json:"totalIntegrations"`
	ActiveIntegrations string `json:"activeIntegrations"`
	ErrorsLast24h      string `json:"errorsLast24h"`
	Disconnected       string `json:"disconnected"`

	// Widgets
	SyncEventsTrend  string `json:"syncEventsTrend"`
	ByProvider       string `json:"byProvider"`
	RecentErrors     string `json:"recentErrors"`
	NoRecentErrors   string `json:"noRecentErrors"`
	NoProviders      string `json:"noProviders"`

	// Quick actions
	QuickConnect          string `json:"quickConnect"`
	QuickTestSync         string `json:"quickTestSync"`
	QuickViewLogs         string `json:"quickViewLogs"`
	QuickConfigureWebhook string `json:"quickConfigureWebhook"`

	// Common
	ViewAll    string `json:"viewAll"`
	AxisEvents string `json:"axisEvents"`

	// Provider table column headers
	ProviderColumn string `json:"providerColumn"`
	StatusColumn   string `json:"statusColumn"`
	LastSyncColumn string `json:"lastSyncColumn"`
}

// DefaultIntegrationLabels returns IntegrationLabels populated with English defaults.
// Consumer apps load translations on top via lyngua's LoadPathIfExists.
func DefaultIntegrationLabels() IntegrationLabels {
	return IntegrationLabels{
		Dashboard: IntegrationDashboardLabels{
			Title:    "Integrations",
			Subtitle: "Provider health, recent sync activity, and webhook status across connected services",

			TotalIntegrations:  "Total Integrations",
			ActiveIntegrations: "Active",
			ErrorsLast24h:      "Errors (24h)",
			Disconnected:       "Disconnected",

			SyncEventsTrend: "Sync Events (7d)",
			ByProvider:      "By Provider",
			RecentErrors:    "Recent Errors",
			NoRecentErrors:  "No recent errors",
			NoProviders:     "No integrations configured",

			QuickConnect:          "Connect Integration",
			QuickTestSync:         "Test Sync",
			QuickViewLogs:         "View Logs",
			QuickConfigureWebhook: "Configure Webhook",

			ViewAll:    "View All",
			AxisEvents: "Events",

			ProviderColumn: "Provider",
			StatusColumn:   "Status",
			LastSyncColumn: "Last Sync",
		},
	}
}
