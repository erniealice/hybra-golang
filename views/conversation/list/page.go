// Package list provides the STAFF conversation inbox view: a thread-list table
// with status filter chips and a perm-gated "New conversation" primary action.
//
// Workspace scoping is enforced by the espyna ListConversations use case; this
// view applies the {status} filter as a presentation-layer predicate over the
// returned rows (Q-MSG-15: no per-client row filter for staff).
package list

import (
	"context"
	"log"

	appcontext "github.com/erniealice/espyna-golang/appcontext"
	conversationpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/communication/conversation"
	pyeza "github.com/erniealice/pyeza-golang"
	"github.com/erniealice/pyeza-golang/route"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"

	convshared "github.com/erniealice/hybra-golang/views/conversation/model"
)

// Deps holds view dependencies for the staff inbox list + table views.
type Deps struct {
	Routes       convshared.ConversationRoutes
	Labels       convshared.ConversationLabels
	CommonLabels pyeza.CommonLabels
	TableLabels  types.TableLabels

	// ListConversations is the espyna use-case closure. Workspace scoping is
	// applied inside the use case.
	ListConversations func(ctx context.Context, req *conversationpb.ListConversationsRequest) (*conversationpb.ListConversationsResponse, error)

	// ClientNameByID resolves a client_id to a display name for the Client
	// column. Nil-safe: when nil the raw client_id is shown.
	ClientNameByID func(ctx context.Context, ids []string) map[string]string

	// FormatTimestamp renders a unix-seconds value into a display string.
	// Nil-safe.
	FormatTimestamp func(unixSec int64) string
}

// PageData is the staff inbox page model.
type PageData struct {
	types.PageData
	ContentTemplate string
	Table           *types.TableConfig
	Filters         []FilterChip
	ActiveFilter    string
	Labels          convshared.ConversationLabels
}

// FilterChip is one inbox filter-chip view-model.
type FilterChip struct {
	Key    string
	Label  string
	URL    string
	Active bool
}

// NewView returns the full-page staff inbox view.
func NewView(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "list") {
			return view.Forbidden("conversation:list")
		}

		statusKey := viewCtx.Request.PathValue("status")
		if statusKey == "" {
			statusKey = convshared.StatusFilterAll
		}

		table, err := deps.buildTable(ctx, statusKey)
		if err != nil {
			log.Printf("conversation inbox: failed to list conversations: %v", err)
			return view.Error(err)
		}

		l := deps.Labels
		pageData := &PageData{
			PageData: types.PageData{
				CacheVersion:   viewCtx.CacheVersion,
				Title:          l.List.Heading,
				CurrentPath:    viewCtx.CurrentPath,
				ActiveNav:      "conversations",
				HeaderTitle:    l.List.Heading,
				HeaderSubtitle: l.List.Subtitle,
				HeaderIcon:     "icon-message-square",
				CommonLabels:   deps.CommonLabels,
			},
			ContentTemplate: "conversation-list-content",
			Table:           table,
			Filters:         deps.buildFilterChips(statusKey),
			ActiveFilter:    statusKey,
			Labels:          l,
		}
		return view.OK("conversation-list", pageData)
	})
}

// NewTableView returns the HTMX table-refresh partial view.
func NewTableView(deps *Deps) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		perms := view.GetUserPermissions(ctx)
		if !perms.Can("conversation", "list") {
			return view.Forbidden("conversation:list")
		}

		statusKey := viewCtx.Request.PathValue("status")
		if statusKey == "" {
			statusKey = convshared.StatusFilterAll
		}

		table, err := deps.buildTable(ctx, statusKey)
		if err != nil {
			log.Printf("conversation inbox table: failed to list conversations: %v", err)
			return view.Error(err)
		}
		return view.OK("table-card", table)
	})
}

// buildTable lists conversations, applies the status filter, and assembles the
// TableConfig.
func (deps *Deps) buildTable(ctx context.Context, statusKey string) (*types.TableConfig, error) {
	resp, err := deps.ListConversations(ctx, &conversationpb.ListConversationsRequest{})
	if err != nil {
		return nil, err
	}
	all := resp.GetData()

	sessionUserID := ""
	if uid, err := appcontext.RequireUserIDFromContext(ctx); err == nil {
		sessionUserID = uid
	}

	filtered := filterByStatusKey(all, statusKey, sessionUserID)

	// Resolve client display names in one batch.
	clientNames := map[string]string{}
	if deps.ClientNameByID != nil {
		ids := make([]string, 0, len(filtered))
		for _, c := range filtered {
			if cid := c.GetClientId(); cid != "" {
				ids = append(ids, cid)
			}
		}
		clientNames = deps.ClientNameByID(ctx, ids)
	}

	rows := deps.buildRows(filtered, clientNames)

	l := deps.Labels
	columns := []types.TableColumn{
		{Key: "subject", Label: l.Columns.Subject},
		{Key: "client", Label: l.Columns.Client},
		{Key: "last_activity", Label: l.Columns.LastActivity, WidthClass: "col-2xl"},
		{Key: "assignee", Label: l.Columns.Assignee, WidthClass: "col-2xl"},
		{Key: "status", Label: l.Columns.Status, WidthClass: "col-2xl"},
	}
	types.ApplyColumnStyles(columns, rows)

	tableConfig := &types.TableConfig{
		ID:                   "conversations-table",
		RefreshURL:           route.ResolveURL(deps.Routes.TableURL, "status", statusKey),
		Columns:              columns,
		Rows:                 rows,
		ShowSearch:           true,
		ShowActions:          true,
		DefaultSortColumn:    "last_activity",
		DefaultSortDirection: "desc",
		Labels:               deps.TableLabels,
		EmptyState: types.TableEmptyState{
			Title:   l.List.EmptyTitle,
			Message: l.List.EmptyMessage,
		},
		PrimaryAction: &types.PrimaryAction{
			Label:     l.Actions.NewConversation,
			ActionURL: deps.Routes.AddURL,
			Icon:      "icon-plus",
		},
	}
	types.ApplyTableSettings(tableConfig)
	return tableConfig, nil
}

// buildRows maps conversations to table rows with per-row Open/Assign/status actions.
func (deps *Deps) buildRows(conversations []*conversationpb.Conversation, clientNames map[string]string) []types.TableRow {
	l := deps.Labels
	rows := make([]types.TableRow, 0, len(conversations))
	for _, c := range conversations {
		id := c.GetId()
		status := c.GetStatus()

		clientCell := c.GetClientId()
		if name, ok := clientNames[c.GetClientId()]; ok && name != "" {
			clientCell = name
		}

		assignee := l.Thread.Unassigned
		if a := c.GetAssignedToUserId(); a != "" {
			assignee = a
		}

		lastActivity := ""
		if deps.FormatTimestamp != nil {
			lastActivity = deps.FormatTimestamp(c.GetLastPostAt())
		}

		actions := []types.TableAction{
			{
				Type:  "view",
				Label: l.Actions.Open,
				URL:   route.ResolveURL(deps.Routes.DetailURL, "id", id),
			},
			{
				Type:        "edit",
				Label:       l.Actions.Assign,
				Action:      "assign",
				URL:         deps.Routes.AssignURL + "?id=" + id,
				DrawerTitle: l.Actions.Assign,
			},
			{
				Type:        "edit",
				Label:       l.Actions.SetStatus,
				Action:      "set-status",
				URL:         deps.Routes.SetStatusURL + "?id=" + id,
				DrawerTitle: l.Actions.SetStatus,
			},
		}

		rows = append(rows, types.TableRow{
			ID: id,
			Cells: []types.TableCell{
				{Type: "text", Value: c.GetSubject()},
				{Type: "text", Value: clientCell},
				{Type: "text", Value: lastActivity},
				{Type: "text", Value: assignee},
				{Type: "badge", Value: convshared.StatusLabel(status, l.Status), Variant: convshared.StatusBadgeVariant(status)},
			},
			DataAttrs: map[string]string{
				"subject": c.GetSubject(),
				"status":  convshared.StatusKey(status),
				"testid":  "conversation-row-" + id,
			},
			Actions: actions,
		})
	}
	return rows
}

// buildFilterChips constructs the inbox filter-chip row.
func (deps *Deps) buildFilterChips(active string) []FilterChip {
	l := deps.Labels.Inbox
	defs := []struct{ key, label string }{
		{convshared.StatusFilterAll, l.FilterAll},
		{convshared.StatusFilterUnassigned, l.FilterUnassigned},
		{convshared.StatusFilterMyQueue, l.FilterMyQueue},
		{convshared.StatusFilterOpen, l.FilterOpen},
		{convshared.StatusFilterInProgress, l.FilterInProgress},
		{convshared.StatusFilterResolved, l.FilterResolved},
		{convshared.StatusFilterClosed, l.FilterClosed},
	}
	chips := make([]FilterChip, 0, len(defs))
	for _, d := range defs {
		chips = append(chips, FilterChip{
			Key:    d.key,
			Label:  d.label,
			URL:    route.ResolveURL(deps.Routes.ListURL, "status", d.key),
			Active: d.key == active,
		})
	}
	return chips
}

// filterByStatusKey applies the {status} segment predicate to the conversation
// slice. "all" = open + in_progress; "unassigned" = no assignee and not closed;
// "my-queue" = assigned to the session user; single-status keys filter exactly.
func filterByStatusKey(in []*conversationpb.Conversation, key, sessionUserID string) []*conversationpb.Conversation {
	if status, ok := convshared.ParseStatusKey(key); ok {
		out := make([]*conversationpb.Conversation, 0, len(in))
		for _, c := range in {
			if c.GetStatus() == status {
				out = append(out, c)
			}
		}
		return out
	}

	out := make([]*conversationpb.Conversation, 0, len(in))
	switch key {
	case convshared.StatusFilterUnassigned:
		for _, c := range in {
			if c.GetAssignedToUserId() == "" &&
				c.GetStatus() != conversationpb.ConversationStatus_CONVERSATION_STATUS_CLOSED {
				out = append(out, c)
			}
		}
	case convshared.StatusFilterMyQueue:
		for _, c := range in {
			if sessionUserID != "" && c.GetAssignedToUserId() == sessionUserID {
				out = append(out, c)
			}
		}
	default: // "all" — open + in_progress
		for _, c := range in {
			switch c.GetStatus() {
			case conversationpb.ConversationStatus_CONVERSATION_STATUS_OPEN,
				conversationpb.ConversationStatus_CONVERSATION_STATUS_IN_PROGRESS:
				out = append(out, c)
			}
		}
	}
	return out
}
