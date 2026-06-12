// Package action holds the HTMX action handlers for the conversation surface:
// new-conversation drawer (add), assign drawer, set-status drawer, the polling
// posts partial, and the composer send endpoint.
//
// All espyna use cases arrive as typed closures — no consumer/* imports.
package action

import (
	"context"

	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
	conversationreadreceiptpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_read_receipt"
	pyeza "github.com/erniealice/pyeza-golang"

	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
)

// Deps holds dependencies shared by every conversation action handler.
type Deps struct {
	Routes       convshared.ConversationRoutes
	Labels       convshared.ConversationLabels
	PostLabels   convshared.ConversationPostLabels
	CommonLabels pyeza.CommonLabels

	// Autocomplete endpoints (staff drawers). Empty hides the field.
	ClientSearchURL   string
	AssigneeSearchURL string

	// Use-case closures (espyna). Optional ones are nil-safe.
	CreateConversation func(ctx context.Context, req *conversationpb.CreateConversationRequest) (*conversationpb.CreateConversationResponse, error)
	ReadConversation   func(ctx context.Context, req *conversationpb.ReadConversationRequest) (*conversationpb.ReadConversationResponse, error)

	// Assign + SetStatus consume the proto UpdateConversationRequest — the
	// espyna AssignConversation / SetConversationStatus use cases dispatch on
	// the mutated field. Optional (drawer hidden when nil).
	AssignConversation    func(ctx context.Context, req *conversationpb.UpdateConversationRequest) (*conversationpb.UpdateConversationResponse, error)
	SetConversationStatus func(ctx context.Context, req *conversationpb.UpdateConversationRequest) (*conversationpb.UpdateConversationResponse, error)

	// Posts list + composer send.
	ListConversationPosts func(ctx context.Context, req *conversationpostpb.ListConversationPostsRequest) (*conversationpostpb.ListConversationPostsResponse, error)
	SendConversationPost  func(ctx context.Context, req *conversationpostpb.CreateConversationPostRequest) (*conversationpostpb.CreateConversationPostResponse, error)

	// Read-receipt high-water-mark upsert. Optional.
	MarkConversationRead func(ctx context.Context, req *conversationreadreceiptpb.CreateConversationReadReceiptRequest) (*conversationreadreceiptpb.CreateConversationReadReceiptResponse, error)

	// NewClientToken generates a composer idempotency token. Nil-safe.
	NewClientToken func() string

	// FormatTimestamp renders unix-seconds into a display string. Nil-safe.
	FormatTimestamp func(unixSec int64) string

	// ActingAsClientID returns the session's acting-as-client scope, or "" for
	// a staff principal. Nil-safe (treated as staff when unset). Supplied by
	// the host via ModuleDeps so the view layer stays decoupled from
	// espyna/consumer (block-decouple invariant — invariant #1).
	ActingAsClientID func(ctx context.Context) string
}

// actingAsClientID is a nil-safe accessor for the optional session scope.
func (d *Deps) actingAsClientID(ctx context.Context) string {
	if d.ActingAsClientID == nil {
		return ""
	}
	return d.ActingAsClientID(ctx)
}
