package action

import (
	"context"
	"log"
	"net/http"

	appcontext "github.com/erniealice/espyna-golang/appcontext"
	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	"github.com/erniealice/pyeza-golang/view"
	"google.golang.org/protobuf/proto"

	convform "github.com/erniealice/hybra-golang/views/conversation/form"
)

// NewAssignAction returns the assign-conversation drawer handler (staff only).
//
//	GET  → render the assign drawer (current assignee + assignee autocomplete).
//	POST → AssignConversation; respond via sheet-response.
func NewAssignAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "update") {
			return view.HTMXError(deps.Labels.Errors.PermissionDenied)
		}
		if deps.AssignConversation == nil {
			return view.HTMXError(deps.Labels.Errors.SaveFailed)
		}

		id := paramID(viewCtx.Request)
		if id == "" {
			return view.HTMXError(deps.Labels.Errors.IDRequired)
		}

		if viewCtx.Request.Method == http.MethodGet {
			currentAssignee := ""
			if deps.ReadConversation != nil {
				if resp, err := deps.ReadConversation(ctx, &conversationpb.ReadConversationRequest{
					Data: &conversationpb.Conversation{Id: id},
				}); err == nil {
					if data := resp.GetData(); len(data) > 0 {
						currentAssignee = data[0].GetAssignedToUserId()
					}
				}
			}
			if currentAssignee == "" {
				currentAssignee = deps.Labels.Thread.Unassigned
			}
			return view.OK("conversation-assign-form", &convform.Data{
				FormAction:        deps.Routes.AssignURL,
				WorkspaceID:       appcontext.GetWorkspaceIDFromContext(ctx),
				ID:                id,
				IsEdit:            true,
				AssigneeSearchURL: deps.AssigneeSearchURL,
				CurrentAssignee:   currentAssignee,
				Labels:            formLabels(deps.Labels),
				CommonLabels:      deps.CommonLabels,
			})
		}

		if err := viewCtx.Request.ParseForm(); err != nil {
			return view.HTMXError(deps.Labels.Errors.InvalidForm)
		}
		assignee := viewCtx.Request.FormValue("assigned_to_user_id")
		if assignee == "" {
			return view.HTMXError(deps.Labels.Errors.SaveFailed)
		}

		_, err := deps.AssignConversation(ctx, &conversationpb.UpdateConversationRequest{
			Data: &conversationpb.Conversation{
				Id:               id,
				AssignedToUserId: proto.String(assignee),
			},
		})
		if err != nil {
			log.Printf("conversation assign: %s failed: %v", id, err)
			return view.HTMXError(err.Error())
		}
		return view.HTMXSuccess("conversations-table")
	})
}

// paramID reads the conversation id from query (?id=) or form value.
func paramID(r *http.Request) string {
	if id := r.URL.Query().Get("id"); id != "" {
		return id
	}
	_ = r.ParseForm()
	return r.FormValue("id")
}
