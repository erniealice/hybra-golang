package action

import (
	"context"
	"log"
	"net/http"
	"strconv"

	appcontext "github.com/erniealice/espyna-golang/appcontext"
	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	"github.com/erniealice/pyeza-golang/view"

	convform "github.com/erniealice/hybra-golang/views/conversation/form"
	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
)

// NewSetStatusAction returns the status-transition drawer handler (staff only).
//
//	GET  → render the set-status drawer (current status + allowed transitions).
//	POST → SetConversationStatus; respond via sheet-response.
func NewSetStatusAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "update") {
			return view.HTMXError(deps.Labels.Errors.PermissionDenied)
		}
		if deps.SetConversationStatus == nil {
			return view.HTMXError(deps.Labels.Errors.SaveFailed)
		}

		id := paramID(viewCtx.Request)
		if id == "" {
			return view.HTMXError(deps.Labels.Errors.IDRequired)
		}

		if viewCtx.Request.Method == http.MethodGet {
			current := conversationpb.ConversationStatus_CONVERSATION_STATUS_UNSPECIFIED
			if deps.ReadConversation != nil {
				if resp, err := deps.ReadConversation(ctx, &conversationpb.ReadConversationRequest{
					Data: &conversationpb.Conversation{Id: id},
				}); err == nil {
					if data := resp.GetData(); len(data) > 0 {
						current = data[0].GetStatus()
					}
				}
			}

			opts := make([]convform.StatusOption, 0, 4)
			for _, t := range convshared.AllowedTransitions(current) {
				opts = append(opts, convform.StatusOption{
					Value: strconv.Itoa(int(t)),
					Label: convshared.StatusLabel(t, deps.Labels.Status),
				})
			}

			return view.OK("conversation-set-status-form", &convform.Data{
				FormAction:         deps.Routes.SetStatusURL,
				WorkspaceID:        appcontext.GetWorkspaceIDFromContext(ctx),
				ID:                 id,
				IsEdit:             true,
				CurrentStatus:      strconv.Itoa(int(current)),
				CurrentStatusLabel: convshared.StatusLabel(current, deps.Labels.Status),
				AllowedTransitions: opts,
				Labels:             formLabels(deps.Labels),
				CommonLabels:       deps.CommonLabels,
			})
		}

		if err := viewCtx.Request.ParseForm(); err != nil {
			return view.HTMXError(deps.Labels.Errors.InvalidForm)
		}
		statusRaw := viewCtx.Request.FormValue("status")
		statusInt, err := strconv.Atoi(statusRaw)
		if err != nil || statusInt == 0 {
			return view.HTMXError(deps.Labels.Errors.InvalidTransition)
		}
		target := conversationpb.ConversationStatus(statusInt)

		_, err = deps.SetConversationStatus(ctx, &conversationpb.UpdateConversationRequest{
			Data: &conversationpb.Conversation{
				Id:     id,
				Status: target,
			},
		})
		if err != nil {
			log.Printf("conversation set-status: %s -> %v failed: %v", id, target, err)
			return view.HTMXError(err.Error())
		}
		return view.HTMXSuccess("conversations-table")
	})
}
