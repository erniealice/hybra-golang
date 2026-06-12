// Package detail provides the STAFF thread-detail view: a thread header with
// status badge + assign button, a scrollable post-bubble list (30s HTMX
// polling), and a simple composer.
package detail

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

// Deps holds view dependencies for the staff thread-detail view.
type Deps struct {
	Routes       convshared.ConversationRoutes
	Labels       convshared.ConversationLabels
	PostLabels   convshared.ConversationPostLabels
	CommonLabels pyeza.CommonLabels

	ReadConversation      func(ctx context.Context, req *conversationpb.ReadConversationRequest) (*conversationpb.ReadConversationResponse, error)
	ListConversationPosts func(ctx context.Context, req *conversationpostpb.ListConversationPostsRequest) (*conversationpostpb.ListConversationPostsResponse, error)

	// NewClientToken generates the idempotency token seeded into the composer
	// (Q-MSG-7). Required non-empty on Send.
	NewClientToken func() string

	// FormatTimestamp renders unix-seconds into a display string. Nil-safe.
	FormatTimestamp func(unixSec int64) string

	// ClientNameByID resolves the conversation's client_id to a display name.
	// Nil-safe.
	ClientNameByID func(ctx context.Context, ids []string) map[string]string
}

// PageData is the staff thread-detail page model.
type PageData struct {
	types.PageData
	ContentTemplate string

	ConversationID string
	Subject        string
	ClientLabel    string
	AssigneeLabel  string
	StatusLabel    string
	StatusVariant  string

	ClientToken string

	Routes     convshared.ConversationRoutes
	Labels     convshared.ConversationLabels
	PostLabels convshared.ConversationPostLabels

	CanUpdate bool

	PostsData convshared.PostsPartialData
}

// NewView returns the staff thread-detail view.
func NewView(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "read") {
			return view.Forbidden("conversation:read")
		}

		id := viewCtx.Request.PathValue("id")
		if id == "" {
			return view.Error(errIDRequired)
		}

		// ReadConversation — workspace scoping + IDOR check inside the use case.
		convResp, err := deps.ReadConversation(ctx, &conversationpb.ReadConversationRequest{
			Data: &conversationpb.Conversation{Id: id},
		})
		if err != nil {
			log.Printf("conversation detail: read %s failed: %v", id, err)
			return view.Error(err)
		}
		data := convResp.GetData()
		if len(data) == 0 {
			return view.Error(errNotFound)
		}
		conv := data[0]

		viewerUserID := ""
		if uid, err := appcontext.RequireUserIDFromContext(ctx); err == nil {
			viewerUserID = uid
		}

		posts := deps.loadPosts(ctx, id, viewerUserID)

		l := deps.Labels
		clientLabel := conv.GetClientId()
		if deps.ClientNameByID != nil {
			if names := deps.ClientNameByID(ctx, []string{conv.GetClientId()}); names != nil {
				if n := names[conv.GetClientId()]; n != "" {
					clientLabel = n
				}
			}
		}

		assigneeLabel := l.Thread.Unassigned
		if a := conv.GetAssignedToUserId(); a != "" {
			assigneeLabel = a
		}

		clientToken := ""
		if deps.NewClientToken != nil {
			clientToken = deps.NewClientToken()
		}

		pageData := &PageData{
			PageData: types.PageData{
				CacheVersion:   viewCtx.CacheVersion,
				Title:          conv.GetSubject(),
				CurrentPath:    viewCtx.CurrentPath,
				ActiveNav:      "conversations",
				HeaderTitle:    conv.GetSubject(),
				HeaderSubtitle: l.Thread.Subtitle,
				HeaderIcon:     "icon-message-square",
				CommonLabels:   deps.CommonLabels,
			},
			ContentTemplate: "conversation-detail-content",
			ConversationID:  id,
			Subject:         conv.GetSubject(),
			ClientLabel:     clientLabel,
			AssigneeLabel:   assigneeLabel,
			StatusLabel:     convshared.StatusLabel(conv.GetStatus(), l.Status),
			StatusVariant:   convshared.StatusBadgeVariant(conv.GetStatus()),
			ClientToken:     clientToken,
			Routes:          deps.Routes,
			Labels:          l,
			PostLabels:      deps.PostLabels,
			CanUpdate:       perms.Can("conversation", "update"),
			PostsData: convshared.PostsPartialData{
				Posts:  posts,
				Labels: deps.PostLabels,
			},
		}
		return view.OK("conversation-detail", pageData)
	})
}

// loadPosts lists posts for the conversation and maps them to bubbles.
func (deps *Deps) loadPosts(ctx context.Context, conversationID, viewerUserID string) []convshared.PostBubble {
	if deps.ListConversationPosts == nil {
		return nil
	}
	resp, err := deps.ListConversationPosts(ctx, &conversationpostpb.ListConversationPostsRequest{})
	if err != nil {
		log.Printf("conversation detail: list posts for %s failed: %v", conversationID, err)
		return nil
	}
	// Filter to this conversation (the use case scopes by workspace/IDOR; the
	// per-thread filter is a presentation predicate).
	scoped := make([]*conversationpostpb.ConversationPost, 0, len(resp.GetData()))
	for _, p := range resp.GetData() {
		if p.GetConversationId() == conversationID {
			scoped = append(scoped, p)
		}
	}
	return convshared.BuildPostBubbles(scoped, viewerUserID, deps.PostLabels, deps.FormatTimestamp)
}
