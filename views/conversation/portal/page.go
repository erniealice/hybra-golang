// Package portal provides the CLIENT-FACING split-panel "Messages" view.
//
// GATED: this surface is built but only registered when AUTHZ_ENFORCE=true AND
// the inherited 20260601 Phase-4 prerequisite is met — the portal/auth
// middleware must populate acting_as_client_id for direct PRINCIPAL_TYPE_CLIENT
// principals. Until then the block registers a 503 stub instead of these
// handlers (see block.go RegisterPortalRoutes wiring). Every handler here ALSO
// fail-closes: it returns Forbidden when acting_as_client_id is empty, so even
// if mis-wired it never leaks cross-client data (Q-MSG-5).
package portal

import (
	"context"
	"log"

	appcontext "github.com/erniealice/espyna-golang/appcontext"
	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
	pyeza "github.com/erniealice/pyeza-golang"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"

	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
)

// Deps holds dependencies for the client portal split-panel view.
type Deps struct {
	Routes       convshared.ConversationRoutes
	Labels       convshared.ConversationLabels
	PostLabels   convshared.ConversationPostLabels
	CommonLabels pyeza.CommonLabels

	ListConversations     func(ctx context.Context, req *conversationpb.ListConversationsRequest) (*conversationpb.ListConversationsResponse, error)
	ReadConversation      func(ctx context.Context, req *conversationpb.ReadConversationRequest) (*conversationpb.ReadConversationResponse, error)
	ListConversationPosts func(ctx context.Context, req *conversationpostpb.ListConversationPostsRequest) (*conversationpostpb.ListConversationPostsResponse, error)

	NewClientToken  func() string
	FormatTimestamp func(unixSec int64) string

	// ActingAsClientID returns the session's acting-as-client scope (required
	// non-empty on this surface — fail-closed). Nil-safe accessor below.
	// Supplied by the host so the view stays decoupled from espyna/consumer.
	ActingAsClientID func(ctx context.Context) string
}

// actingAsClientID is a nil-safe accessor for the session client scope.
func (d *Deps) actingAsClientID(ctx context.Context) string {
	if d.ActingAsClientID == nil {
		return ""
	}
	return d.ActingAsClientID(ctx)
}

// ThreadRow is a left-panel thread-list row view-model.
type ThreadRow struct {
	ID       string
	Subject  string
	Time     string
	Active   bool
	IsUnread bool
}

// PageData is the portal split-panel page model.
type PageData struct {
	types.PageData
	ContentTemplate string

	Threads     []ThreadRow
	ActiveID    string
	HasActive   bool
	Subject     string
	StatusLabel string
	ClientToken string

	Routes     convshared.ConversationRoutes
	Labels     convshared.ConversationLabels
	PostLabels convshared.ConversationPostLabels

	PostsData convshared.PostsPartialData
}

// NewPageView returns the client portal split-panel "Messages" view.
//
// Fail-closed: returns Forbidden when acting_as_client_id is empty (Q-MSG-5).
func NewPageView(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "list") {
			return view.Forbidden("conversation:list")
		}
		// HARD fail-closed: a direct-client portal request MUST carry a
		// populated acting_as_client_id. Until the 20260601 Phase-4 middleware
		// wires it, this returns 403 rather than leaking workspace-wide data.
		if deps.actingAsClientID(ctx) == "" {
			return view.Forbidden("conversation:list")
		}

		activeID := viewCtx.Request.URL.Query().Get("id")

		threads, err := deps.buildThreads(ctx, activeID)
		if err != nil {
			log.Printf("conversation portal: list failed: %v", err)
			return view.Error(err)
		}

		l := deps.Labels
		pageData := &PageData{
			PageData: types.PageData{
				CacheVersion:   viewCtx.CacheVersion,
				Title:          l.List.Title,
				CurrentPath:    viewCtx.CurrentPath,
				ActiveNav:      "conversations",
				HeaderTitle:    l.List.Title,
				HeaderSubtitle: l.Thread.Subtitle,
				HeaderIcon:     "icon-message-square",
				CommonLabels:   deps.CommonLabels,
			},
			ContentTemplate: "conversation-portal-content",
			Threads:         threads,
			ActiveID:        activeID,
			Routes:          deps.Routes,
			Labels:          l,
			PostLabels:      deps.PostLabels,
		}

		if activeID != "" {
			deps.fillActiveThread(ctx, pageData, activeID)
		}

		return view.OK("conversation-portal", pageData)
	})
}

// NewPostsView returns the portal polling posts-partial handler.
func NewPostsView(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation_post", "read") {
			return view.Forbidden("conversation_post:read")
		}
		if deps.actingAsClientID(ctx) == "" {
			return view.Forbidden("conversation_post:read")
		}
		conversationID := viewCtx.Request.URL.Query().Get("id")
		if conversationID == "" {
			return view.HTMXError(deps.Labels.Errors.IDRequired)
		}
		bubbles := deps.loadBubbles(ctx, conversationID)
		return view.OK("conversation-posts-partial", convshared.PostsPartialData{
			Posts:  bubbles,
			Labels: deps.PostLabels,
		})
	})
}

func (deps *Deps) buildThreads(ctx context.Context, activeID string) ([]ThreadRow, error) {
	resp, err := deps.ListConversations(ctx, &conversationpb.ListConversationsRequest{})
	if err != nil {
		return nil, err
	}
	rows := make([]ThreadRow, 0, len(resp.GetData()))
	for _, c := range resp.GetData() {
		t := ""
		if deps.FormatTimestamp != nil {
			t = deps.FormatTimestamp(c.GetLastPostAt())
		}
		rows = append(rows, ThreadRow{
			ID:      c.GetId(),
			Subject: c.GetSubject(),
			Time:    t,
			Active:  c.GetId() == activeID,
		})
	}
	return rows, nil
}

func (deps *Deps) fillActiveThread(ctx context.Context, pd *PageData, activeID string) {
	if deps.ReadConversation != nil {
		if resp, err := deps.ReadConversation(ctx, &conversationpb.ReadConversationRequest{
			Data: &conversationpb.Conversation{Id: activeID},
		}); err == nil {
			if data := resp.GetData(); len(data) > 0 {
				conv := data[0]
				pd.HasActive = true
				pd.Subject = conv.GetSubject()
				pd.StatusLabel = convshared.StatusLabel(conv.GetStatus(), deps.Labels.Status)
			}
		}
	}
	if deps.NewClientToken != nil {
		pd.ClientToken = deps.NewClientToken()
	}
	pd.PostsData = convshared.PostsPartialData{
		Posts:  deps.loadBubbles(ctx, activeID),
		Labels: deps.PostLabels,
	}
}

func (deps *Deps) loadBubbles(ctx context.Context, conversationID string) []convshared.PostBubble {
	if deps.ListConversationPosts == nil {
		return nil
	}
	viewerUserID := ""
	if uid, err := appcontext.RequireUserIDFromContext(ctx); err == nil {
		viewerUserID = uid
	}
	resp, err := deps.ListConversationPosts(ctx, &conversationpostpb.ListConversationPostsRequest{})
	if err != nil {
		log.Printf("conversation portal: list posts for %s failed: %v", conversationID, err)
		return nil
	}
	scoped := make([]*conversationpostpb.ConversationPost, 0, len(resp.GetData()))
	for _, p := range resp.GetData() {
		if p.GetConversationId() == conversationID {
			scoped = append(scoped, p)
		}
	}
	return convshared.BuildPostBubbles(scoped, viewerUserID, deps.PostLabels, deps.FormatTimestamp)
}
