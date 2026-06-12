package conversation

import (
	"context"

	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
	conversationreadreceiptpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_read_receipt"
	pyeza "github.com/erniealice/pyeza-golang"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"

	convaction "github.com/erniealice/hybra-golang/views/conversation/action"
	convdetail "github.com/erniealice/hybra-golang/views/conversation/detail"
	convlist "github.com/erniealice/hybra-golang/views/conversation/list"
	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
	convportal "github.com/erniealice/hybra-golang/views/conversation/portal"
)

// ModuleDeps holds all dependencies for the conversation view module.
//
// Closure signatures use the REAL espyna use-case request/response types:
//   - AssignConversation / SetConversationStatus consume UpdateConversationRequest
//     (the espyna use cases dispatch on the mutated field — there is no distinct
//     AssignConversationRequest / SetConversationStatusRequest proto message).
//   - SendConversationPost consumes CreateConversationPostRequest.
//   - MarkConversationRead consumes CreateConversationReadReceiptRequest.
//
// Optional closures are nil-safe: the corresponding surface degrades (the assign
// / set-status drawers refuse with a clean error; mark-read becomes a no-op).
type ModuleDeps struct {
	Routes       convshared.ConversationRoutes
	CommonLabels pyeza.CommonLabels
	TableLabels  types.TableLabels
	Labels       convshared.ConversationLabels
	PostLabels   convshared.ConversationPostLabels

	// Staff inbox + thread detail.
	ListConversations     func(ctx context.Context, req *conversationpb.ListConversationsRequest) (*conversationpb.ListConversationsResponse, error)
	ReadConversation      func(ctx context.Context, req *conversationpb.ReadConversationRequest) (*conversationpb.ReadConversationResponse, error)
	CreateConversation    func(ctx context.Context, req *conversationpb.CreateConversationRequest) (*conversationpb.CreateConversationResponse, error)
	AssignConversation    func(ctx context.Context, req *conversationpb.UpdateConversationRequest) (*conversationpb.UpdateConversationResponse, error)
	SetConversationStatus func(ctx context.Context, req *conversationpb.UpdateConversationRequest) (*conversationpb.UpdateConversationResponse, error)

	// Posts + composer.
	ListConversationPosts func(ctx context.Context, req *conversationpostpb.ListConversationPostsRequest) (*conversationpostpb.ListConversationPostsResponse, error)
	SendConversationPost  func(ctx context.Context, req *conversationpostpb.CreateConversationPostRequest) (*conversationpostpb.CreateConversationPostResponse, error)

	// Read-receipt.
	MarkConversationRead func(ctx context.Context, req *conversationreadreceiptpb.CreateConversationReadReceiptRequest) (*conversationreadreceiptpb.CreateConversationReadReceiptResponse, error)

	// Cross-module helpers (all nil-safe).
	// ClientNameByID resolves client_ids to display names for the inbox + detail.
	ClientNameByID func(ctx context.Context, ids []string) map[string]string
	// FormatTimestamp renders unix-seconds into a display string.
	FormatTimestamp func(unixSec int64) string
	// NewClientToken generates a composer idempotency token (Q-MSG-7).
	NewClientToken func() string

	// ActingAsClientID returns the session's acting-as-client scope, or "" for
	// a staff principal. Supplied by the host (service-admin, Phase 4) so the
	// view layer stays decoupled from espyna/consumer (invariant #1). Until the
	// 20260601 Phase-4 middleware populates acting_as_client_id for direct
	// clients this stays nil → the portal surface fail-closes (403).
	ActingAsClientID func(ctx context.Context) string

	// Autocomplete endpoints for the staff new-conversation drawer.
	ClientSearchURL   string
	AssigneeSearchURL string
}

// Module holds all constructed conversation views.
type Module struct {
	routes convshared.ConversationRoutes

	StaffList   view.View
	StaffTable  view.View
	StaffDetail view.View
	Add         view.View
	Assign      view.View
	SetStatus   view.View
	Posts       view.View
	Send        view.View
	MarkRead    view.View

	// Portal (client principal). Non-nil only when ListConversations is wired;
	// registered behind the AUTHZ_ENFORCE + Phase-4 gate (see RegisterPortalRoutes).
	PortalPage  view.View
	PortalPosts view.View
}

// NewModule constructs every conversation sub-view from the typed deps.
func NewModule(deps *ModuleDeps) *Module {
	actionDeps := &convaction.Deps{
		Routes:                deps.Routes,
		Labels:                deps.Labels,
		PostLabels:            deps.PostLabels,
		CommonLabels:          deps.CommonLabels,
		ClientSearchURL:       deps.ClientSearchURL,
		AssigneeSearchURL:     deps.AssigneeSearchURL,
		CreateConversation:    deps.CreateConversation,
		ReadConversation:      deps.ReadConversation,
		AssignConversation:    deps.AssignConversation,
		SetConversationStatus: deps.SetConversationStatus,
		ListConversationPosts: deps.ListConversationPosts,
		SendConversationPost:  deps.SendConversationPost,
		MarkConversationRead:  deps.MarkConversationRead,
		NewClientToken:        deps.NewClientToken,
		FormatTimestamp:       deps.FormatTimestamp,
		ActingAsClientID:      deps.ActingAsClientID,
	}

	listDeps := &convlist.Deps{
		Routes:            deps.Routes,
		Labels:            deps.Labels,
		CommonLabels:      deps.CommonLabels,
		TableLabels:       deps.TableLabels,
		ListConversations: deps.ListConversations,
		ClientNameByID:    deps.ClientNameByID,
		FormatTimestamp:   deps.FormatTimestamp,
	}

	detailDeps := &convdetail.Deps{
		Routes:                deps.Routes,
		Labels:                deps.Labels,
		PostLabels:            deps.PostLabels,
		CommonLabels:          deps.CommonLabels,
		ReadConversation:      deps.ReadConversation,
		ListConversationPosts: deps.ListConversationPosts,
		NewClientToken:        deps.NewClientToken,
		FormatTimestamp:       deps.FormatTimestamp,
		ClientNameByID:        deps.ClientNameByID,
	}

	m := &Module{
		routes:      deps.Routes,
		StaffList:   convlist.NewView(listDeps),
		StaffTable:  convlist.NewTableView(listDeps),
		StaffDetail: convdetail.NewView(detailDeps),
		Add:         convaction.NewAddAction(actionDeps),
		Assign:      convaction.NewAssignAction(actionDeps),
		SetStatus:   convaction.NewSetStatusAction(actionDeps),
		Posts:       convaction.NewPostsAction(actionDeps),
		Send:        convaction.NewSendAction(actionDeps),
		MarkRead:    convaction.NewMarkReadAction(actionDeps),
	}

	// Construct the portal views when the list use case is available. They are
	// only REGISTERED behind the gate, but constructing them here keeps the
	// 503-stub branch in block.go simple (nil PortalPage = not ready).
	if deps.ListConversations != nil {
		portalDeps := &convportal.Deps{
			Routes:                deps.Routes,
			Labels:                deps.Labels,
			PostLabels:            deps.PostLabels,
			CommonLabels:          deps.CommonLabels,
			ListConversations:     deps.ListConversations,
			ReadConversation:      deps.ReadConversation,
			ListConversationPosts: deps.ListConversationPosts,
			NewClientToken:        deps.NewClientToken,
			FormatTimestamp:       deps.FormatTimestamp,
			ActingAsClientID:      deps.ActingAsClientID,
		}
		m.PortalPage = convportal.NewPageView(portalDeps)
		m.PortalPosts = convportal.NewPostsView(portalDeps)
	}

	return m
}

// RegisterRoutes registers all STAFF routes (pages + action endpoints).
// Portal routes are registered separately via RegisterPortalRoutes, gated by
// the caller.
func (m *Module) RegisterRoutes(r view.RouteRegistrar) {
	// Staff inbox + thread detail.
	r.GET(m.routes.ListURL, m.StaffList)
	r.GET(m.routes.TableURL, m.StaffTable)
	r.GET(m.routes.DetailURL, m.StaffDetail)

	// Drawers (GET = form, POST = submit).
	r.GET(m.routes.AddURL, m.Add)
	r.POST(m.routes.AddURL, m.Add)
	r.GET(m.routes.AssignURL, m.Assign)
	r.POST(m.routes.AssignURL, m.Assign)
	r.GET(m.routes.SetStatusURL, m.SetStatus)
	r.POST(m.routes.SetStatusURL, m.SetStatus)

	// Post sub-endpoints.
	r.GET(m.routes.PostsURL, m.Posts)
	r.POST(m.routes.SendURL, m.Send)
	r.POST(m.routes.MarkReadURL, m.MarkRead)
}

// RegisterPortalRoutes registers the client-facing portal routes.
//
// PRECONDITION (owned by the caller — see block.go):
//  1. AUTHZ_ENFORCE=true.
//  2. The portal/auth middleware populates acting_as_client_id for direct
//     PRINCIPAL_TYPE_CLIENT principals (inherited 20260601 Phase-4 dependency).
//
// When PortalPage is nil (use cases unwired) this is a no-op; the caller is
// expected to register a 503 stub so the URL exists for discovery tests.
// Even when registered, every portal handler ALSO fail-closes on an empty
// acting_as_client_id, so a mis-wire cannot leak cross-client data.
func (m *Module) RegisterPortalRoutes(r view.RouteRegistrar) {
	if m.PortalPage == nil {
		return
	}
	r.GET(m.routes.PortalListURL, m.PortalPage)
	r.GET(m.routes.PortalPostsURL, m.PortalPosts)
	// Send + mark-read are SHARED with the staff surface — the same handlers
	// (m.Send / m.MarkRead) serve both, principal-scoped at the use-case layer.
	// They are ALREADY registered by RegisterRoutes, which always runs before
	// this; re-registering here panics the Go 1.22 ServeMux on a duplicate
	// pattern ("POST /action/conversation_post/send" registered twice). So the
	// portal deliberately registers ONLY its own GET pages.
}

// PortalReady reports whether the portal views were constructed (use cases
// wired). The caller pairs this with the AUTHZ_ENFORCE + Phase-4 env gate.
func (m *Module) PortalReady() bool { return m.PortalPage != nil }
