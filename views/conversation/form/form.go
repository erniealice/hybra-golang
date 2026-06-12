// Package form holds the flat template-data shapes for the conversation
// drawer forms (new-conversation, assign, set-status).
//
// Per the drawer-form-subpackage convention these types carry NO Deps, NO
// context.Context, and NO espyna/esqyma imports — they are pure presentation
// structs handed to the HTML templates. The action handlers in
// views/conversation/action build these from the use-case responses.
package form

// Data is the flat shape every conversation drawer template receives.
type Data struct {
	// FormAction is the hx-post target for the form.
	FormAction string
	// WorkspaceID is injected by the ViewAdapter for the action_workspace_guard
	// middleware ({{actionForm .FormAction .WorkspaceID}} in the template).
	WorkspaceID string
	// ID is populated for edit/assign/status drawers (the conversation id);
	// empty for a brand-new conversation.
	ID string
	// IsEdit distinguishes create vs. mutate drawers.
	IsEdit bool

	// IsClientPrincipal controls which fields render. When true the client +
	// assignee autocompletes are hidden (the client is stamped from session).
	IsClientPrincipal bool

	// Autocomplete search endpoints (staff only). Empty hides the field.
	ClientSearchURL   string
	AssigneeSearchURL string

	// Pre-fill values.
	Subject       string
	ClientID      string
	ClientLabel   string
	AssigneeID    string
	AssigneeLabel string
	ReferenceID   string

	// Status-drawer fields.
	CurrentStatus      string
	CurrentStatusLabel string
	CurrentAssignee    string
	AllowedTransitions []StatusOption

	// Labels + common labels (any avoids an import cycle on pyeza.CommonLabels).
	Labels       Labels
	CommonLabels any
}

// Labels is the flat label struct used by all conversation drawer templates.
type Labels struct {
	SectionTitle         string
	SubjectLabel         string
	SubjectPlaceholder   string
	ClientLabel          string
	ClientPlaceholder    string
	AssigneeLabel        string
	AssigneePlaceholder  string
	LinkLabel            string
	LinkPlaceholder      string
	MessageLabel         string
	MessagePlaceholder   string
	CurrentStatusLabel   string
	NewStatusLabel       string
	CurrentAssigneeLabel string
}

// StatusOption is one allowed status-transition target in the set-status drawer.
type StatusOption struct {
	Value string
	Label string
}
