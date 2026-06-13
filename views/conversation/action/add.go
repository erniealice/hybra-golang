package action

import (
	"context"
	"log"
	"net/http"

	"github.com/erniealice/espyna-golang/shared/identity"
	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
	"github.com/erniealice/pyeza-golang/view"
	"google.golang.org/protobuf/proto"

	convform "github.com/erniealice/hybra-golang/views/conversation/form"
	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
)

// NewAddAction returns the new-conversation drawer handler.
//
//	GET  → render the blank drawer form.
//	POST → CreateConversation (+ optional first post), respond via sheet-response.
func NewAddAction(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "create") {
			return view.HTMXError(deps.Labels.Errors.PermissionDenied)
		}

		// A client principal (acting-as-client scope) hides the client +
		// assignee pickers; the use case stamps client_id from session.
		isClientPrincipal := deps.actingAsClientID(ctx) != ""

		if viewCtx.Request.Method == http.MethodGet {
			return view.OK("conversation-drawer-form", &convform.Data{
				FormAction:        deps.Routes.AddURL,
				WorkspaceID:       identity.Must(ctx).WorkspaceID,
				IsClientPrincipal: isClientPrincipal,
				ClientSearchURL:   deps.ClientSearchURL,
				AssigneeSearchURL: deps.AssigneeSearchURL,
				Labels:            formLabels(deps.Labels),
				CommonLabels:      deps.CommonLabels,
			})
		}

		if err := viewCtx.Request.ParseForm(); err != nil {
			return view.HTMXError(deps.Labels.Errors.InvalidForm)
		}
		r := viewCtx.Request

		subject := r.FormValue("subject")
		if subject == "" {
			return view.HTMXError(deps.Labels.Errors.SubjectRequired)
		}
		clientID := r.FormValue("client_id")
		if !isClientPrincipal && clientID == "" {
			return view.HTMXError(deps.Labels.Errors.ClientRequired)
		}

		conv := &conversationpb.Conversation{
			Subject: subject,
		}
		if clientID != "" {
			conv.ClientId = clientID
		}
		if assignee := r.FormValue("assigned_to_user_id"); assignee != "" {
			conv.AssignedToUserId = proto.String(assignee)
		}
		if refID := r.FormValue("reference_entity_id"); refID != "" {
			conv.ReferenceEntityId = proto.String(refID)
			conv.ReferenceEntityType = proto.String("request")
		}

		createResp, err := deps.CreateConversation(ctx, &conversationpb.CreateConversationRequest{Data: conv})
		if err != nil {
			log.Printf("conversation add: create failed: %v", err)
			return view.HTMXError(err.Error())
		}

		newID := ""
		if data := createResp.GetData(); len(data) > 0 {
			newID = data[0].GetId()
		}

		// Optional first message — sent as a separate post (the Conversation
		// header carries no body). Best-effort: a send failure does not roll
		// back the created conversation.
		if body := r.FormValue("body"); body != "" && newID != "" && deps.SendConversationPost != nil {
			token := ""
			if deps.NewClientToken != nil {
				token = deps.NewClientToken()
			}
			_, sendErr := deps.SendConversationPost(ctx, &conversationpostpb.CreateConversationPostRequest{
				Data: &conversationpostpb.ConversationPost{
					ConversationId: newID,
					Body:           body,
					ClientToken:    proto.String(token),
				},
			})
			if sendErr != nil {
				log.Printf("conversation add: first-post send failed for %s: %v", newID, sendErr)
			}
		}

		return view.HTMXSuccess("conversations-table")
	})
}

// formLabels maps ConversationLabels into the flat drawer Labels struct.
func formLabels(l convshared.ConversationLabels) convform.Labels {
	return convform.Labels{
		SectionTitle:         l.Form.SectionTitle,
		SubjectLabel:         l.Form.SubjectLabel,
		SubjectPlaceholder:   l.Form.SubjectPlaceholder,
		ClientLabel:          l.Form.ClientLabel,
		ClientPlaceholder:    l.Form.ClientPlaceholder,
		AssigneeLabel:        l.Form.AssigneeLabel,
		AssigneePlaceholder:  l.Form.AssigneePlaceholder,
		LinkLabel:            l.Form.LinkLabel,
		LinkPlaceholder:      l.Form.LinkPlaceholder,
		MessageLabel:         l.Form.MessageLabel,
		MessagePlaceholder:   l.Form.MessagePlaceholder,
		CurrentStatusLabel:   l.Form.CurrentStatusLabel,
		NewStatusLabel:       l.Form.NewStatusLabel,
		CurrentAssigneeLabel: l.Form.CurrentAssigneeLabel,
	}
}
