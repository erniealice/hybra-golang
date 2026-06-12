// Package model holds the shared view-models, status helpers, and the
// label + route CONTRACT types (contract.go) for the conversation surface
// (Plan-4, 2026-06-03). It is a LEAF package: it imports only esqyma proto,
// never the conversation view sub-packages — this breaks the import cycle that
// would otherwise arise from both the root conversation package and its
// list/detail/action/portal sub-packages needing these helpers and types.
package model

import (
	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	conversationpostpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation_post"
)

// StatusFilter values (the {status} path segment on the staff inbox).
const (
	StatusFilterAll        = "all"
	StatusFilterUnassigned = "unassigned"
	StatusFilterMyQueue    = "my-queue"
	StatusFilterOpen       = "open"
	StatusFilterInProgress = "in-progress"
	StatusFilterResolved   = "resolved"
	StatusFilterClosed     = "closed"
)

// StatusLabel maps a ConversationStatus enum to its human-readable label.
func StatusLabel(s conversationpb.ConversationStatus, l ConversationStatusLabels) string {
	switch s {
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN:
		return l.Open
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS:
		return l.InProgress
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED:
		return l.Resolved
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED:
		return l.Closed
	default:
		return l.Unknown
	}
}

// StatusBadgeVariant maps a ConversationStatus to a pyeza badge Status value.
// Values are restricted to the badge component's supported set
// (active | inactive | prospect | draft | archived | pending | success |
// warning | error) so each renders with a themed style.
func StatusBadgeVariant(s conversationpb.ConversationStatus) string {
	switch s {
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN:
		return "pending"
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS:
		return "active"
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED:
		return "success"
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED:
		return "archived"
	default:
		return "inactive"
	}
}

// StatusKey returns the lowercase status key used in the {status} path segment.
func StatusKey(s conversationpb.ConversationStatus) string {
	switch s {
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN:
		return StatusFilterOpen
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS:
		return StatusFilterInProgress
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED:
		return StatusFilterResolved
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED:
		return StatusFilterClosed
	default:
		return StatusFilterAll
	}
}

// ParseStatusKey maps a {status} path segment to a ConversationStatus enum.
// Returns (status, true) for a single-status key, or (UNSPECIFIED, false) for
// the multi-status / queue filters (all, unassigned, my-queue) which the
// caller resolves with additional predicates.
func ParseStatusKey(key string) (conversationpb.ConversationStatus, bool) {
	switch key {
	case StatusFilterOpen:
		return conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN, true
	case StatusFilterInProgress:
		return conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS, true
	case StatusFilterResolved:
		return conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED, true
	case StatusFilterClosed:
		return conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED, true
	default:
		return conversationpb.ConversationStatus_CONVERSATION_STATUS_UNSPECIFIED, false
	}
}

// AllowedTransitions returns the valid status-transition targets from the
// current status, enforcing the transition map (pages.md §F). The UI hides
// illegal transitions; the espyna SetConversationStatus use case validates the
// same map server-side.
func AllowedTransitions(from conversationpb.ConversationStatus) []conversationpb.ConversationStatus {
	switch from {
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN:
		return []conversationpb.ConversationStatus{
			conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS,
			conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED,
			conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED,
		}
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS:
		return []conversationpb.ConversationStatus{
			conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN,
			conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED,
			conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED,
		}
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_RESOLVED:
		return []conversationpb.ConversationStatus{
			conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN,
			conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS,
			conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED,
		}
	case conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED:
		// Closed → Open only (re-open).
		return []conversationpb.ConversationStatus{
			conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN,
		}
	default:
		return nil
	}
}

// IsTransitionAllowed reports whether from→to is a legal status transition.
func IsTransitionAllowed(from, to conversationpb.ConversationStatus) bool {
	for _, t := range AllowedTransitions(from) {
		if t == to {
			return true
		}
	}
	return false
}

// PostBubble is the flat view-model for a single message bubble in the
// conversation-posts-partial template. Direction is resolved by the caller
// (IsMine) from the viewer's perspective (pages.md §D.3).
type PostBubble struct {
	ID           string
	Body         string
	SenderLabel  string
	SentAt       string
	IsMine       bool
	IsClientSent bool
}

// PostsPartialData is the data the conversation-posts-partial template receives.
type PostsPartialData struct {
	Posts  []PostBubble
	Labels ConversationPostLabels
}

// BuildPostBubbles maps proto posts into view-model bubbles, resolving bubble
// direction from the viewer's user id (Q-MSG-13). A post authored by the
// current viewer renders right-aligned (.us); everything else renders
// left-aligned (.them).
func BuildPostBubbles(
	posts []*conversationpostpb.ConversationPost,
	viewerUserID string,
	labels ConversationPostLabels,
	formatTimestamp func(unixSec int64) string,
) []PostBubble {
	out := make([]PostBubble, 0, len(posts))
	for _, p := range posts {
		isMine := viewerUserID != "" && p.GetSenderUserId() == viewerUserID
		isClient := p.GetSenderPrincipalType() == conversationpostpb.SenderPrincipalType_SENDER_PRINCIPAL_TYPE_CLIENT

		var senderLabel string
		switch {
		case isMine:
			senderLabel = labels.Bubble.You
		case isClient:
			senderLabel = labels.Bubble.Client
		default:
			senderLabel = labels.Bubble.Staff
		}

		sentAt := ""
		if formatTimestamp != nil {
			sentAt = formatTimestamp(p.GetSentAt())
		}

		out = append(out, PostBubble{
			ID:           p.GetId(),
			Body:         p.GetBody(),
			SenderLabel:  senderLabel,
			SentAt:       sentAt,
			IsMine:       isMine,
			IsClientSent: isClient,
		})
	}
	return out
}
