package action

import (
	"context"
	"log"

	appcontext "github.com/erniealice/espyna-golang/appcontext"
	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
	"github.com/erniealice/pyeza-golang/view"

	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
)

// NewPostsAction returns the polling posts-partial handler. It renders ONLY the
// conversation-posts-partial template (the inner bubbles), which the thread
// view polls every 30s into #conversation-post-list (hx-swap="innerHTML").
func NewPostsAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation_post", "read") {
			return view.Forbidden("conversation_post:read")
		}

		conversationID := viewCtx.Request.URL.Query().Get("id")
		if conversationID == "" {
			return view.HTMXError(deps.Labels.Errors.IDRequired)
		}

		viewerUserID := ""
		if uid, err := appcontext.RequireUserIDFromContext(ctx); err == nil {
			viewerUserID = uid
		}

		var bubbles []convshared.PostBubble
		if deps.ListConversationPosts != nil {
			resp, err := deps.ListConversationPosts(ctx, &conversationpostpb.ListConversationPostsRequest{})
			if err != nil {
				log.Printf("conversation posts: list for %s failed: %v", conversationID, err)
				return view.Error(err)
			}
			scoped := make([]*conversationpostpb.ConversationPost, 0, len(resp.GetData()))
			for _, p := range resp.GetData() {
				if p.GetConversationId() == conversationID {
					scoped = append(scoped, p)
				}
			}
			bubbles = convshared.BuildPostBubbles(scoped, viewerUserID, deps.PostLabels, deps.FormatTimestamp)
		}

		return view.OK("conversation-posts-partial", convshared.PostsPartialData{
			Posts:  bubbles,
			Labels: deps.PostLabels,
		})
	})
}
