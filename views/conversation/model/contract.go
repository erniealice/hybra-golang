// contract.go — conversation label + route contract types for the hybra
// communication/conversation cross-cutting surface (Plan-4, 2026-06-03).
//
// Relocated 2026-06-12 from the entydad root package (view-package-placement.md
// OCID: communication/conversation is a hybra cross-cutting concern, NOT an
// entydad OCID-identity surface — manifest thread TC). The struct shapes, json
// tags, English defaults, and route paths are BYTE-IDENTICAL to the former
// entydad.Conversation* / entydad.ConversationRoutes / entydad route constants
// so the surface is behavior-preserving (same lyngua JSON keys, same URLs).
//
// They live in the `model` LEAF package (not the parent `conversation` package)
// because both the parent conversation view and its list/detail/action/portal
// sub-packages reference these types; the leaf keeps them import-cycle free —
// the same role this package already played for the status helpers.
package model

// Conversation route constants — secure messaging / ticketing (Plan-4, 2026-06-03).
// Staff surface: /app/conversations/* (pages) + /action/conversation* (HTMX).
// Client portal surface: /portal/conversations — gated behind AUTHZ_ENFORCE
// + the inherited 20260601 Phase-4 acting_as_client_id prerequisite (see block.go).
const (
	ConversationListURL        = "/app/conversations/list/{status}"
	ConversationTableURL       = "/action/conversation/table/{status}"
	ConversationDetailURL      = "/app/conversations/detail/{id}"
	ConversationAddURL         = "/action/conversation/add"
	ConversationAssignURL      = "/action/conversation/assign"
	ConversationSetStatusURL   = "/action/conversation/set-status"
	ConversationPostsURL       = "/action/conversation/posts"
	ConversationSendURL        = "/action/conversation_post/send"
	ConversationMarkReadURL    = "/action/conversation/mark-read"
	ConversationPortalListURL  = "/portal/conversations"
	ConversationPortalPostsURL = "/action/conversation/portal-posts"
)

// ---------------------------------------------------------------------------
// ConversationRoutes
// ---------------------------------------------------------------------------

// ConversationRoutes holds all URL constants for the conversation domain view
// module (secure messaging / ticketing, Plan-4 2026-06-03).
//
// Staff routes follow the standard /app/ (pages) + /action/ (HTMX) split.
// Portal routes are client-facing and only registered behind the AUTHZ_ENFORCE
// gate + the inherited 20260601 Phase-4 acting_as_client_id prerequisite.
type ConversationRoutes struct {
	// Staff surface
	ListURL      string `json:"list_url"`       // /app/conversations/list/{status}
	TableURL     string `json:"table_url"`      // /action/conversation/table/{status}
	DetailURL    string `json:"detail_url"`     // /app/conversations/detail/{id}
	AddURL       string `json:"add_url"`        // /action/conversation/add
	AssignURL    string `json:"assign_url"`     // /action/conversation/assign
	SetStatusURL string `json:"set_status_url"` // /action/conversation/set-status
	PostsURL     string `json:"posts_url"`      // /action/conversation/posts
	SendURL      string `json:"send_url"`       // /action/conversation_post/send
	MarkReadURL  string `json:"mark_read_url"`  // /action/conversation/mark-read

	// Client portal surface (gated)
	PortalListURL  string `json:"portal_list_url"`  // /portal/conversations
	PortalPostsURL string `json:"portal_posts_url"` // /action/conversation/portal-posts
}

// DefaultConversationRoutes returns ConversationRoutes populated from the
// package-level route constants.
func DefaultConversationRoutes() ConversationRoutes {
	return ConversationRoutes{
		ListURL:        ConversationListURL,
		TableURL:       ConversationTableURL,
		DetailURL:      ConversationDetailURL,
		AddURL:         ConversationAddURL,
		AssignURL:      ConversationAssignURL,
		SetStatusURL:   ConversationSetStatusURL,
		PostsURL:       ConversationPostsURL,
		SendURL:        ConversationSendURL,
		MarkReadURL:    ConversationMarkReadURL,
		PortalListURL:  ConversationPortalListURL,
		PortalPostsURL: ConversationPortalPostsURL,
	}
}

// RouteMap returns a map of dot-notation keys to route paths.
func (r ConversationRoutes) RouteMap() map[string]string {
	return map[string]string{
		"conversation.list":         r.ListURL,
		"conversation.table":        r.TableURL,
		"conversation.detail":       r.DetailURL,
		"conversation.add":          r.AddURL,
		"conversation.assign":       r.AssignURL,
		"conversation.set_status":   r.SetStatusURL,
		"conversation.posts":        r.PostsURL,
		"conversation.send":         r.SendURL,
		"conversation.mark_read":    r.MarkReadURL,
		"conversation.portal_list":  r.PortalListURL,
		"conversation.portal_posts": r.PortalPostsURL,
	}
}

// ===========================================================================
// Conversation labels — secure messaging / ticketing (Plan-4, 2026-06-03)
//
// Loaded from translations/en/{tier}/conversation.json (root key "conversation")
// and conversation_post.json (root key "conversationPost") via LoadPathIfExists.
// All fields are nil-safe: DefaultConversationLabels() pre-populates English so a
// missing JSON file does not produce empty strings in the UI.
// ===========================================================================

// ConversationLabels is the top-level label struct for the conversation surface.
type ConversationLabels struct {
	List    ConversationListLabels    `json:"list"`
	Inbox   ConversationInboxLabels   `json:"inbox"`
	Thread  ConversationThreadLabels  `json:"thread"`
	Status  ConversationStatusLabels  `json:"status"`
	Actions ConversationActionLabels  `json:"actions"`
	Form    ConversationFormLabels    `json:"form"`
	Columns ConversationColumnLabels  `json:"columns"`
	Confirm ConversationConfirmLabels `json:"confirm"`
	Errors  ConversationErrorLabels   `json:"errors"`
}

// ConversationListLabels — staff inbox + portal thread-list headings.
type ConversationListLabels struct {
	Heading      string `json:"heading"`
	Subtitle     string `json:"subtitle"`
	Title        string `json:"title"`
	NewButton    string `json:"newButton"`
	EmptyTitle   string `json:"emptyTitle"`
	EmptyMessage string `json:"emptyMessage"`
}

// ConversationInboxLabels — staff filter chips.
type ConversationInboxLabels struct {
	FilterAll        string `json:"filterAll"`
	FilterUnassigned string `json:"filterUnassigned"`
	FilterMyQueue    string `json:"filterMyQueue"`
	FilterOpen       string `json:"filterOpen"`
	FilterInProgress string `json:"filterInProgress"`
	FilterResolved   string `json:"filterResolved"`
	FilterClosed     string `json:"filterClosed"`
}

// ConversationThreadLabels — thread-detail header + meta.
type ConversationThreadLabels struct {
	BackToInbox   string `json:"backToInbox"`
	Assignee      string `json:"assignee"`
	Unassigned    string `json:"unassigned"`
	Client        string `json:"client"`
	Created       string `json:"created"`
	LastActivity  string `json:"lastActivity"`
	ViewRequest   string `json:"viewRequest"`
	Subtitle      string `json:"subtitle"`
	EmptyTitle    string `json:"emptyTitle"`
	EmptySubtitle string `json:"emptySubtitle"`
}

// ConversationStatusLabels — human-readable status badge labels keyed by enum.
type ConversationStatusLabels struct {
	Open       string `json:"open"`
	InProgress string `json:"inProgress"`
	Resolved   string `json:"resolved"`
	Closed     string `json:"closed"`
	Unknown    string `json:"unknown"`
}

// ConversationActionLabels — action button labels.
type ConversationActionLabels struct {
	NewConversation string `json:"newConversation"`
	Open            string `json:"open"`
	Assign          string `json:"assign"`
	MarkResolved    string `json:"markResolved"`
	Close           string `json:"close"`
	Reopen          string `json:"reopen"`
	SetStatus       string `json:"setStatus"`
	Send            string `json:"send"`
}

// ConversationFormLabels — new-conversation / assign / status drawer fields.
type ConversationFormLabels struct {
	SectionTitle         string `json:"sectionTitle"`
	SubjectLabel         string `json:"subjectLabel"`
	SubjectPlaceholder   string `json:"subjectPlaceholder"`
	ClientLabel          string `json:"clientLabel"`
	ClientPlaceholder    string `json:"clientPlaceholder"`
	AssigneeLabel        string `json:"assigneeLabel"`
	AssigneePlaceholder  string `json:"assigneePlaceholder"`
	LinkLabel            string `json:"linkLabel"`
	LinkPlaceholder      string `json:"linkPlaceholder"`
	MessageLabel         string `json:"messageLabel"`
	MessagePlaceholder   string `json:"messagePlaceholder"`
	CurrentStatusLabel   string `json:"currentStatusLabel"`
	NewStatusLabel       string `json:"newStatusLabel"`
	CurrentAssigneeLabel string `json:"currentAssigneeLabel"`
}

// ConversationColumnLabels — staff inbox table headers.
type ConversationColumnLabels struct {
	Subject      string `json:"subject"`
	Client       string `json:"client"`
	LastActivity string `json:"lastActivity"`
	Assignee     string `json:"assignee"`
	Status       string `json:"status"`
}

// ConversationConfirmLabels — confirm-dialog copy for status transitions.
type ConversationConfirmLabels struct {
	ResolveTitle   string `json:"resolveTitle"`
	ResolveMessage string `json:"resolveMessage"`
	CloseTitle     string `json:"closeTitle"`
	CloseMessage   string `json:"closeMessage"`
	ReopenTitle    string `json:"reopenTitle"`
	ReopenMessage  string `json:"reopenMessage"`
}

// ConversationErrorLabels — error strings surfaced via HTMX error toast.
type ConversationErrorLabels struct {
	PermissionDenied  string `json:"permissionDenied"`
	NotFound          string `json:"notFound"`
	InvalidForm       string `json:"invalidForm"`
	SubjectRequired   string `json:"subjectRequired"`
	ClientRequired    string `json:"clientRequired"`
	MessageRequired   string `json:"messageRequired"`
	InvalidTransition string `json:"invalidTransition"`
	IDRequired        string `json:"idRequired"`
	SaveFailed        string `json:"saveFailed"`
}

// ConversationPostLabels is the label struct for the post composer + bubbles.
// Loaded from conversation_post.json (root key "conversationPost").
type ConversationPostLabels struct {
	Composer ConversationComposerLabels  `json:"composer"`
	Bubble   ConversationBubbleLabels    `json:"bubble"`
	Subtitle string                      `json:"subtitle"`
	Empty    string                      `json:"empty"`
	Errors   ConversationPostErrorLabels `json:"errors"`
}

// ConversationComposerLabels — reply composer.
type ConversationComposerLabels struct {
	Placeholder string `json:"placeholder"`
	Send        string `json:"send"`
	Attach      string `json:"attach"`
}

// ConversationBubbleLabels — sender role labels.
type ConversationBubbleLabels struct {
	You    string `json:"you"`
	Staff  string `json:"staff"`
	Client string `json:"client"`
}

// ConversationPostErrorLabels — post-specific errors.
type ConversationPostErrorLabels struct {
	EmptyBody    string `json:"emptyBody"`
	MissingToken string `json:"missingToken"`
	SendFailed   string `json:"sendFailed"`
}

// DefaultConversationLabels returns English defaults for the conversation
// surface. Override per business type via conversation.json.
func DefaultConversationLabels() ConversationLabels {
	return ConversationLabels{
		List: ConversationListLabels{
			Heading:      "Conversations",
			Subtitle:     "Secure messaging with your clients",
			Title:        "Messages",
			NewButton:    "New",
			EmptyTitle:   "No conversations yet",
			EmptyMessage: "Start a new conversation to message a client.",
		},
		Inbox: ConversationInboxLabels{
			FilterAll:        "All open",
			FilterUnassigned: "Unassigned",
			FilterMyQueue:    "My queue",
			FilterOpen:       "Open",
			FilterInProgress: "In progress",
			FilterResolved:   "Resolved",
			FilterClosed:     "Closed",
		},
		Thread: ConversationThreadLabels{
			BackToInbox:   "Back to inbox",
			Assignee:      "Assigned to",
			Unassigned:    "Unassigned",
			Client:        "Client",
			Created:       "Created",
			LastActivity:  "Last activity",
			ViewRequest:   "View request",
			Subtitle:      "Secure messaging. Every conversation is logged.",
			EmptyTitle:    "Select a conversation",
			EmptySubtitle: "Choose a thread from the list to view messages.",
		},
		Status: ConversationStatusLabels{
			Open:       "Open",
			InProgress: "In progress",
			Resolved:   "Resolved",
			Closed:     "Closed",
			Unknown:    "Unknown",
		},
		Actions: ConversationActionLabels{
			NewConversation: "New conversation",
			Open:            "Open",
			Assign:          "Assign",
			MarkResolved:    "Mark resolved",
			Close:           "Close",
			Reopen:          "Reopen",
			SetStatus:       "Change status",
			Send:            "Send",
		},
		Form: ConversationFormLabels{
			SectionTitle:         "Conversation details",
			SubjectLabel:         "Subject",
			SubjectPlaceholder:   "What is this about?",
			ClientLabel:          "Client",
			ClientPlaceholder:    "Search clients…",
			AssigneeLabel:        "Assign to",
			AssigneePlaceholder:  "Search staff…",
			LinkLabel:            "Linked request (optional)",
			LinkPlaceholder:      "e.g. REQ-0091",
			MessageLabel:         "Message",
			MessagePlaceholder:   "Type your first message…",
			CurrentStatusLabel:   "Current status",
			NewStatusLabel:       "New status",
			CurrentAssigneeLabel: "Currently",
		},
		Columns: ConversationColumnLabels{
			Subject:      "Conversation",
			Client:       "Client",
			LastActivity: "Last activity",
			Assignee:     "Assigned",
			Status:       "Status",
		},
		Confirm: ConversationConfirmLabels{
			ResolveTitle:   "Mark resolved",
			ResolveMessage: "Mark this conversation as resolved?",
			CloseTitle:     "Close conversation",
			CloseMessage:   "Close this conversation? It can be reopened later.",
			ReopenTitle:    "Reopen conversation",
			ReopenMessage:  "Reopen this conversation?",
		},
		Errors: ConversationErrorLabels{
			PermissionDenied:  "You don't have permission to perform this action.",
			NotFound:          "Conversation not found.",
			InvalidForm:       "Invalid form data.",
			SubjectRequired:   "A subject is required.",
			ClientRequired:    "Please select a client.",
			MessageRequired:   "A message is required.",
			InvalidTransition: "That status change is not allowed.",
			IDRequired:        "A conversation id is required.",
			SaveFailed:        "Could not save. Please try again.",
		},
	}
}

// DefaultConversationPostLabels returns English defaults for the composer /
// bubble surface. Override per business type via conversation_post.json.
func DefaultConversationPostLabels() ConversationPostLabels {
	return ConversationPostLabels{
		Composer: ConversationComposerLabels{
			Placeholder: "Reply…",
			Send:        "Send",
			Attach:      "Attach",
		},
		Bubble: ConversationBubbleLabels{
			You:    "You",
			Staff:  "Staff",
			Client: "Client",
		},
		Subtitle: "Secure messaging. Every conversation is logged.",
		Empty:    "No messages yet.",
		Errors: ConversationPostErrorLabels{
			EmptyBody:    "Message cannot be empty.",
			MissingToken: "Missing idempotency token. Please refresh and try again.",
			SendFailed:   "Could not send your message. Please try again.",
		},
	}
}
