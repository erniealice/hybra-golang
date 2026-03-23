package auditlog

import "context"

// AuditOps bundles the cross-cutting function dependencies for audit trail
// operations. Embed this struct in detail view Deps structs alongside
// attachment.AttachmentOps.
//
// Go struct embedding promotes these fields, so deps.ListAuditHistory
// resolves directly at call sites.
//
// Embed in all detail views — audit history is universal for detail pages.
//
// NOTE: The function signature uses hybra-local types (not espyna infraports)
// because espyna's port types live under internal/ and cannot be imported from
// outside the espyna module. Domain adapters bridge by converting
// infraports.ListAuditResponse → auditlog.ListAuditResponse at the wiring site.
type AuditOps struct {
	// ListAuditHistory returns paginated audit entries for an entity.
	// Called by the audit-history tab handler on detail pages.
	ListAuditHistory func(ctx context.Context, req *ListAuditRequest) (*ListAuditResponse, error)
	// Future: ListFieldChanges, CreateAuditNote, ExportAuditCSV
}

// ListAuditRequest is the query parameters for listing audit entries.
// Mirrors infraports.ListAuditRequest — kept in sync manually.
type ListAuditRequest struct {
	WorkspaceID string
	EntityType  string
	EntityID    string
	Limit       int
	CursorToken string // opaque cursor for keyset pagination
}

// ListAuditResponse is the paginated result.
// Mirrors infraports.ListAuditResponse — kept in sync manually.
type ListAuditResponse struct {
	Entries    []AuditEntryView
	HasNext    bool
	NextCursor string
}

// AuditEntryView is a single audit entry with its field changes, shaped for
// presentation. Mirrors infraports.AuditEntryResult — kept in sync manually.
type AuditEntryView struct {
	ID             string
	ActorID        string
	ActorType      int32
	Action         int32
	PermissionCode string
	UseCase        string
	FieldCount     int32
	OccurredAt     string // RFC3339 UTC
	FieldChanges   []AuditFieldChangeView
}

// AuditFieldChangeView represents one field-level change.
// Mirrors infraports.AuditFieldChange — kept in sync manually.
type AuditFieldChangeView struct {
	FieldName string
	FieldType int32
	OldValue  string
	NewValue  string
}
