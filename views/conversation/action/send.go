package action

import (
	"context"
	"log"

	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
	conversationreadreceiptpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_read_receipt"
	"github.com/erniealice/pyeza-golang/view"
	"google.golang.org/protobuf/proto"
)

// NewSendAction returns the composer POST handler. It sends a post via
// SendConversationPost (which requires a non-empty client_token, Q-MSG-7) and
// responds via sheet-response (hx-swap="none"); the thread view's 30s poll
// surfaces the new bubble on the next tick.
func NewSendAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation_post", "create") {
			return view.HTMXError(deps.Labels.Errors.PermissionDenied)
		}
		if deps.SendConversationPost == nil {
			return view.HTMXError(deps.PostLabels.Errors.SendFailed)
		}

		if err := viewCtx.Request.ParseForm(); err != nil {
			return view.HTMXError(deps.Labels.Errors.InvalidForm)
		}
		r := viewCtx.Request

		conversationID := r.FormValue("conversation_id")
		if conversationID == "" {
			return view.HTMXError(deps.Labels.Errors.IDRequired)
		}
		body := r.FormValue("body")
		if body == "" {
			return view.HTMXError(deps.PostLabels.Errors.EmptyBody)
		}
		// client_token is REQUIRED (Q-MSG-7 / codex H3). Reject empty here so
		// the user sees a clean message instead of the raw use-case error.
		clientToken := r.FormValue("client_token")
		if clientToken == "" {
			return view.HTMXError(deps.PostLabels.Errors.MissingToken)
		}

		_, err := deps.SendConversationPost(ctx, &conversationpostpb.CreateConversationPostRequest{
			Data: &conversationpostpb.ConversationPost{
				ConversationId: conversationID,
				Body:           body,
				ClientToken:    proto.String(clientToken),
			},
		})
		if err != nil {
			log.Printf("conversation send: %s failed: %v", conversationID, err)
			return view.HTMXError(err.Error())
		}

		return view.HTMXSuccess("conversation-post-list")
	})
}

// NewMarkReadAction returns the mark-read handler (fires once on thread open).
// Upserts the read-receipt high-water mark; nil-safe (no-op when the use case
// is unwired). Always responds 200 with no body (hx-swap="none").
func NewMarkReadAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation_post", "read") {
			return view.Forbidden("conversation_post:read")
		}
		if deps.MarkConversationRead == nil {
			return view.ViewResult{StatusCode: 200}
		}

		_ = viewCtx.Request.ParseForm()
		conversationID := viewCtx.Request.FormValue("conversation_id")
		if conversationID == "" {
			conversationID = viewCtx.Request.URL.Query().Get("conversation_id")
		}
		if conversationID == "" {
			return view.ViewResult{StatusCode: 200}
		}

		if _, err := deps.MarkConversationRead(ctx, &conversationreadreceiptpb.CreateConversationReadReceiptRequest{
			Data: &conversationreadreceiptpb.ConversationReadReceipt{
				ConversationId: conversationID,
			},
		}); err != nil {
			// Mark-read is best-effort; log and still return 200 so the thread
			// open is never blocked by a receipt failure.
			log.Printf("conversation mark-read: %s failed: %v", conversationID, err)
		}
		return view.ViewResult{StatusCode: 200}
	})
}
