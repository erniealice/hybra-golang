package template

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/erniealice/pyeza-golang/route"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"

	documenttemplatepb "github.com/erniealice/esqyma/pkg/schema/v1/domain/ledger/document_template"
)

// DefaultMaxUploadBytes is the fallback when Config.MaxUploadBytes is 0 (10 MB).
const DefaultMaxUploadBytes int64 = 10 << 20

// DocxContentType is the MIME type for .docx files.
const DocxContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

// Config parameterizes the generic template management handlers for a specific document purpose.
type Config struct {
	DocumentPurpose    string   // "invoice", "purchase_order", "receiving_report"
	AllowedExtensions  []string // [".docx"]
	AllowedContentType string   // docx MIME type
	StoragePrefix      string   // "templates/invoice"
	BucketName         string   // "templates"
	MaxUploadBytes     int64    // 0 = DefaultMaxUploadBytes

	// Page URLs
	ListURL       string
	UploadURL     string
	DeleteURL     string
	SetDefaultURL string

	// UI
	Labels       Labels
	CommonLabels any
	TableLabels  types.TableLabels
	ActiveNav    string // "sales", "purchases"
	PageIcon     string // "icon-file-text"

	// Proto CRUD (injected from composition root)
	ListDocumentTemplates  func(ctx context.Context, req *documenttemplatepb.ListDocumentTemplatesRequest) (*documenttemplatepb.ListDocumentTemplatesResponse, error)
	CreateDocumentTemplate func(ctx context.Context, req *documenttemplatepb.CreateDocumentTemplateRequest) (*documenttemplatepb.CreateDocumentTemplateResponse, error)
	UpdateDocumentTemplate func(ctx context.Context, req *documenttemplatepb.UpdateDocumentTemplateRequest) (*documenttemplatepb.UpdateDocumentTemplateResponse, error)
	DeleteDocumentTemplate func(ctx context.Context, req *documenttemplatepb.DeleteDocumentTemplateRequest) (*documenttemplatepb.DeleteDocumentTemplateResponse, error)

	// Storage
	UploadFile func(ctx context.Context, bucket, key string, content []byte, contentType string) error
}

// Labels holds UI text for the template management feature.
type Labels struct {
	PageTitle      string `json:"pageTitle"`
	Caption        string `json:"caption"`
	UploadTemplate string `json:"uploadTemplate"`
	TemplateName   string `json:"templateName"`
	TemplateType   string `json:"templateType"`
	Purpose        string `json:"purpose"`
	SetDefault     string `json:"setDefault"`
	Delete         string `json:"delete"`
	DefaultBadge   string `json:"defaultBadge"`
	EmptyTitle     string `json:"emptyTitle"`
	EmptyMessage   string `json:"emptyMessage"`
	UploadSuccess  string `json:"uploadSuccess"`
	DeleteConfirm  string `json:"deleteConfirm"`
}

// DefaultLabels returns English defaults for quick prototyping.
func DefaultLabels() Labels {
	return Labels{
		PageTitle:      "Document Templates",
		Caption:        "Manage document templates for generating reports and documents.",
		UploadTemplate: "Upload Template",
		TemplateName:   "Template Name",
		TemplateType:   "Type",
		Purpose:        "Purpose",
		SetDefault:     "Set Default",
		Delete:         "Delete",
		DefaultBadge:   "Default",
		EmptyTitle:     "No templates",
		EmptyMessage:   "Upload document templates to get started.",
		UploadSuccess:  "Template uploaded successfully.",
		DeleteConfirm:  "Are you sure you want to delete this template?",
	}
}

// TemplateData holds display-friendly fields for a single document template.
type TemplateData struct {
	ID              string
	Name            string
	TemplateType    string
	DocumentPurpose string
	OriginalFile    string
	FileSizeBytes   int64
	IsDefault       bool
	SetDefaultURL   string
}

// PageData holds the data for the template list page.
type PageData struct {
	types.PageData
	ContentTemplate string
	Table           *types.TableConfig
}

// UploadFormData is the template data for the upload drawer form.
type UploadFormData struct {
	FormAction   string
	Labels       Labels
	CommonLabels any
	AcceptTypes  string // e.g. ".docx" — for the file input accept attribute
}

func (c *Config) maxBytes() int64 {
	if c.MaxUploadBytes > 0 {
		return c.MaxUploadBytes
	}
	return DefaultMaxUploadBytes
}

func (c *Config) bucketName() string {
	if c.BucketName != "" {
		return c.BucketName
	}
	return "templates"
}

func (c *Config) acceptTypes() string {
	if len(c.AllowedExtensions) > 0 {
		return strings.Join(c.AllowedExtensions, ",")
	}
	return ".docx"
}

// NewListView creates the document template list view.
func NewListView(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		templates := loadTemplateList(ctx, cfg)
		l := cfg.Labels

		columns := templateColumns(l)
		rows := buildTemplateRows(templates, l, cfg)
		types.ApplyColumnStyles(columns, rows)

		tableConfig := &types.TableConfig{
			ID:          "templates-table",
			RefreshURL:  cfg.ListURL,
			Columns:     columns,
			Rows:        rows,
			ShowSearch:  true,
			ShowActions: true,
			ShowEntries: true,
			Labels:      cfg.TableLabels,
			EmptyState: types.TableEmptyState{
				Title:   l.EmptyTitle,
				Message: l.EmptyMessage,
			},
			PrimaryAction: &types.PrimaryAction{
				Label:     l.UploadTemplate,
				ActionURL: cfg.UploadURL,
				Icon:      "icon-upload",
			},
		}
		types.ApplyTableSettings(tableConfig)

		pageData := &PageData{
			PageData: types.PageData{
				CacheVersion:   viewCtx.CacheVersion,
				Title:          l.PageTitle,
				CurrentPath:    viewCtx.CurrentPath,
				ActiveNav:      cfg.ActiveNav,
				HeaderTitle:    l.PageTitle,
				HeaderSubtitle: l.Caption,
				HeaderIcon:     cfg.PageIcon,
				CommonLabels:   cfg.CommonLabels,
			},
			ContentTemplate: "template-list-content",
			Table:           tableConfig,
		}

		return view.OK("template-list", pageData)
	})
}

// NewUploadAction creates a dual-purpose handler: GET = drawer form, POST = upload file.
func NewUploadAction(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		if viewCtx.Request.Method == http.MethodGet {
			return view.OK("template-upload-drawer-form", &UploadFormData{
				FormAction:   cfg.UploadURL,
				Labels:       cfg.Labels,
				CommonLabels: cfg.CommonLabels,
				AcceptTypes:  cfg.acceptTypes(),
			})
		}

		// POST — upload template
		if cfg.UploadFile == nil || cfg.CreateDocumentTemplate == nil {
			log.Printf("Template upload deps not configured for %s", cfg.DocumentPurpose)
			return htmxError("template upload not configured")
		}

		err := viewCtx.Request.ParseMultipartForm(32 << 20)
		if err != nil {
			log.Printf("Failed to parse multipart form: %v", err)
			return htmxError("failed to parse upload")
		}

		// Get template name (required)
		name := viewCtx.Request.FormValue("name")
		if name == "" {
			return htmxError("template name is required")
		}

		// Get the uploaded file
		fh, header, err := viewCtx.Request.FormFile("template_file")
		if err != nil {
			log.Printf("Failed to get uploaded file: %v", err)
			return htmxError("no file provided")
		}
		defer fh.Close()

		// Validate file size
		maxBytes := cfg.maxBytes()
		if header.Size > maxBytes {
			return htmxError(fmt.Sprintf("file too large: %d bytes (max %d)", header.Size, maxBytes))
		}

		// Validate content type
		ct := header.Header.Get("Content-Type")
		if cfg.AllowedContentType != "" && ct != cfg.AllowedContentType {
			return htmxError(fmt.Sprintf("invalid file type: only %s files are accepted", cfg.acceptTypes()))
		}

		// Read file content
		content, err := io.ReadAll(fh)
		if err != nil {
			log.Printf("Failed to read uploaded file: %v", err)
			return htmxError("failed to read file")
		}

		// Generate a new UUID for the object ID and database ID.
		newID := uuid.New().String()

		// Determine file extension from uploaded filename
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".docx"
		}

		// Determine template type from file extension
		templateType := strings.TrimPrefix(ext, ".")

		// Generate a safe storage object key using the new UUID.
		objectKey := fmt.Sprintf("%s/%s%s", cfg.StoragePrefix, newID, ext)

		bucketName := cfg.bucketName()

		// Upload to storage
		err = cfg.UploadFile(ctx, bucketName, objectKey, content, ct)
		if err != nil {
			log.Printf("Failed to upload template: %v", err)
			return htmxError("failed to upload template")
		}

		// Create DB record
		fileSize := header.Size
		originalFilename := header.Filename
		_, err = cfg.CreateDocumentTemplate(ctx, &documenttemplatepb.CreateDocumentTemplateRequest{
			Data: &documenttemplatepb.DocumentTemplate{
				Id:               newID,
				Name:             name,
				TemplateType:     templateType,
				DocumentPurpose:  cfg.DocumentPurpose,
				StorageContainer: &bucketName,
				StorageKey:       &objectKey,
				OriginalFilename: &originalFilename,
				FileSizeBytes:    &fileSize,
				Status:           "active",
				Active:           true,
			},
		})
		if err != nil {
			log.Printf("Failed to create document template record: %v", err)
			return htmxError("failed to save template")
		}

		// Redirect back to list page to show updated list
		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Trigger":  `{"formSuccess":true}`,
				"HX-Redirect": cfg.ListURL,
			},
		}
	})
}

// NewDeleteAction creates a POST handler to delete a document template.
func NewDeleteAction(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		if cfg.DeleteDocumentTemplate == nil {
			return htmxError("template delete not configured")
		}

		viewCtx.Request.ParseForm()
		templateID := viewCtx.Request.FormValue("template_id")
		if templateID == "" {
			return htmxError("template_id is required")
		}

		_, err := cfg.DeleteDocumentTemplate(ctx, &documenttemplatepb.DeleteDocumentTemplateRequest{
			Data: &documenttemplatepb.DocumentTemplate{Id: templateID},
		})
		if err != nil {
			log.Printf("Failed to delete document template %s: %v", templateID, err)
			return htmxError("failed to delete template")
		}

		// Redirect back to list page to show updated list
		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Trigger":  `{"formSuccess":true}`,
				"HX-Redirect": cfg.ListURL,
			},
		}
	})
}

// NewSetDefaultAction creates a POST handler for setting a template as the default.
// It unsets any existing default for the same document purpose, then sets the target.
func NewSetDefaultAction(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		if cfg.UpdateDocumentTemplate == nil || cfg.ListDocumentTemplates == nil {
			return htmxError("template update not configured")
		}

		targetID := viewCtx.Request.PathValue("id")
		if targetID == "" {
			return htmxError("template id is required")
		}

		// List all templates to find and unset any existing default
		resp, err := cfg.ListDocumentTemplates(ctx, &documenttemplatepb.ListDocumentTemplatesRequest{})
		if err != nil {
			log.Printf("Failed to list document templates: %v", err)
			return htmxError("failed to list templates")
		}

		falseVal := false
		trueVal := true

		for _, t := range resp.GetData() {
			if !t.GetActive() || t.GetDocumentPurpose() != cfg.DocumentPurpose {
				continue
			}

			if t.GetIsDefault() && t.GetId() != targetID {
				// Unset existing default
				_, err := cfg.UpdateDocumentTemplate(ctx, &documenttemplatepb.UpdateDocumentTemplateRequest{
					Data: &documenttemplatepb.DocumentTemplate{
						Id:        t.GetId(),
						IsDefault: &falseVal,
					},
				})
				if err != nil {
					log.Printf("Failed to unset default on template %s: %v", t.GetId(), err)
				}
			}
		}

		// Set the target template as default
		_, err = cfg.UpdateDocumentTemplate(ctx, &documenttemplatepb.UpdateDocumentTemplateRequest{
			Data: &documenttemplatepb.DocumentTemplate{
				Id:        targetID,
				IsDefault: &trueVal,
			},
		})
		if err != nil {
			log.Printf("Failed to set default on template %s: %v", targetID, err)
			return htmxError("failed to set default template")
		}

		// Redirect back to list page to show updated list
		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Redirect": cfg.ListURL,
			},
		}
	})
}

// BuildTable creates a TableConfig for displaying document templates.
func BuildTable(templates []TemplateData, cfg *Config) *types.TableConfig {
	l := cfg.Labels

	columns := templateColumns(l)
	rows := buildTemplateRows(templates, l, cfg)
	types.ApplyColumnStyles(columns, rows)

	return &types.TableConfig{
		ID:      "templates-table",
		Columns: columns,
		Rows:    rows,
		Labels:  cfg.TableLabels,
		EmptyState: types.TableEmptyState{
			Title:   l.EmptyTitle,
			Message: l.EmptyMessage,
		},
	}
}

func templateColumns(l Labels) []types.TableColumn {
	return []types.TableColumn{
		{Key: "name", Label: l.TemplateName, Sortable: true},
		{Key: "type", Label: l.TemplateType, Sortable: true, Width: "120px"},
		{Key: "purpose", Label: l.Purpose, Sortable: true, Width: "120px"},
		{Key: "status", Label: l.DefaultBadge, Sortable: true, Width: "120px"},
	}
}

func buildTemplateRows(templates []TemplateData, l Labels, cfg *Config) []types.TableRow {
	rows := []types.TableRow{}
	for _, t := range templates {
		actions := []types.TableAction{}
		if !t.IsDefault {
			actions = append(actions, types.TableAction{
				Type:           "activate",
				Label:          l.SetDefault,
				Action:         "set-default",
				URL:            t.SetDefaultURL,
				ConfirmTitle:   l.SetDefault,
				ConfirmMessage: fmt.Sprintf("Set \"%s\" as the default template?", t.Name),
			})
		}
		actions = append(actions, types.TableAction{
			Type:           "delete",
			Label:          l.Delete,
			Action:         "delete",
			URL:            cfg.DeleteURL,
			ItemName:       t.Name,
			ConfirmTitle:   l.Delete,
			ConfirmMessage: l.DeleteConfirm,
		})

		statusValue := ""
		statusVariant := ""
		if t.IsDefault {
			statusValue = l.DefaultBadge
			statusVariant = "info"
		}

		rows = append(rows, types.TableRow{
			ID: t.ID,
			Cells: []types.TableCell{
				{Type: "text", Value: t.Name},
				{Type: "text", Value: t.TemplateType},
				{Type: "text", Value: t.DocumentPurpose},
				{Type: "badge", Value: statusValue, Variant: statusVariant},
			},
			DataAttrs: map[string]string{
				"name":    t.Name,
				"type":    t.TemplateType,
				"purpose": t.DocumentPurpose,
				"status":  statusValue,
			},
			Actions: actions,
		})
	}
	return rows
}

// loadTemplateList loads all active document templates for the configured purpose.
func loadTemplateList(ctx context.Context, cfg *Config) []TemplateData {
	if cfg.ListDocumentTemplates == nil {
		return nil
	}

	resp, err := cfg.ListDocumentTemplates(ctx, &documenttemplatepb.ListDocumentTemplatesRequest{})
	if err != nil {
		log.Printf("Failed to list document templates: %v", err)
		return nil
	}

	var templates []TemplateData
	for _, t := range resp.GetData() {
		if !t.GetActive() {
			continue
		}
		if t.GetDocumentPurpose() != cfg.DocumentPurpose {
			continue
		}
		templates = append(templates, TemplateData{
			ID:              t.GetId(),
			Name:            t.GetName(),
			TemplateType:    t.GetTemplateType(),
			DocumentPurpose: t.GetDocumentPurpose(),
			OriginalFile:    t.GetOriginalFilename(),
			FileSizeBytes:   t.GetFileSizeBytes(),
			IsDefault:       t.GetIsDefault(),
			SetDefaultURL:   route.ResolveURL(cfg.SetDefaultURL, "id", t.GetId()),
		})
	}
	return templates
}

// htmxError returns an HTMX-compatible error response.
func htmxError(msg string) view.ViewResult {
	return view.ViewResult{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"HX-Trigger": fmt.Sprintf(`{"formError":"%s"}`, msg),
		},
	}
}
