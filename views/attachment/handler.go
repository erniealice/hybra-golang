package attachment

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	attachmentpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/document/attachment"
	"github.com/erniealice/pyeza-golang/route"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"
)

// DefaultMaxUploadBytes is the fallback when Config.MaxUploadBytes is 0 (10 MB).
const DefaultMaxUploadBytes int64 = 10 << 20

// Config parameterizes the generic attachment handlers for a specific entity.
type Config struct {
	EntityType     string // e.g. "product", "client", "asset"
	BucketName     string // storage bucket (default: "attachments")
	MaxUploadBytes int64  // 0 = use DefaultMaxUploadBytes
	UploadURL      string // POST form action URL pattern (may contain {id})
	DeleteURL      string // POST delete action URL pattern (may contain {id})
	RedirectURL    string // page to redirect to after action
	Labels         Labels
	CommonLabels   any
	TableLabels    types.TableLabels

	// NewID generates a unique identifier for new attachments.
	NewID func() string

	// Storage operation (injected from composition root)
	UploadFile func(ctx context.Context, bucket, key string, content []byte, contentType string) error

	// Data operations (backed by protobuf use cases)
	ListAttachments  func(ctx context.Context, moduleKey, foreignKey string) (*attachmentpb.ListAttachmentsResponse, error)
	CreateAttachment func(ctx context.Context, req *attachmentpb.CreateAttachmentRequest) (*attachmentpb.CreateAttachmentResponse, error)
	DeleteAttachment func(ctx context.Context, req *attachmentpb.DeleteAttachmentRequest) (*attachmentpb.DeleteAttachmentResponse, error)
}

// Labels holds UI text for the attachment feature.
type Labels struct {
	TabLabel     string // "Attachments"
	UploadTitle  string // "Upload Attachment"
	FileName     string // "File Name"
	FileInput    string // "Select File"
	Description  string // "Description"
	FileType     string // "Type"
	FileSize     string // "Size"
	UploadedAt   string // "Uploaded"
	UploadedBy   string // "Uploaded By"
	EmptyTitle   string // "No attachments"
	EmptyMessage string // "Upload files to attach them to this record."
	Delete       string // "Delete"
	Upload       string // "Upload"
}

// DefaultLabels returns English defaults for quick prototyping.
func DefaultLabels() Labels {
	return Labels{
		TabLabel:     "Attachments",
		UploadTitle:  "Upload Attachment",
		FileName:     "File Name",
		FileInput:    "Select File",
		Description:  "Description (optional)",
		FileType:     "Type",
		FileSize:     "Size",
		UploadedAt:   "Uploaded",
		UploadedBy:   "Uploaded By",
		EmptyTitle:   "No attachments",
		EmptyMessage: "Upload files to attach them to this record.",
		Delete:       "Delete",
		Upload:       "Upload",
	}
}

func (c *Config) maxBytes() int64 {
	if c.MaxUploadBytes > 0 {
		return c.MaxUploadBytes
	}
	return DefaultMaxUploadBytes
}

func (c *Config) newID() string {
	if c.NewID != nil {
		return c.NewID()
	}
	return fmt.Sprintf("att-%d", 0)
}

// UploadFormData is the template data for the upload drawer form.
type UploadFormData struct {
	FormAction   string
	Labels       Labels
	CommonLabels any
	MaxFileSize  int64
	EntityType   string
	EntityID     string
}

// NewUploadAction creates a dual-purpose handler: GET = drawer form, POST = upload file.
func NewUploadAction(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		entityID := viewCtx.Request.PathValue("id")

		if viewCtx.Request.Method == http.MethodGet {
			return view.OK("attachment-upload-drawer-form", &UploadFormData{
				FormAction:   route.ResolveURL(cfg.UploadURL, "id", entityID),
				Labels:       cfg.Labels,
				CommonLabels: nil,
				MaxFileSize:  cfg.maxBytes(),
				EntityType:   cfg.EntityType,
				EntityID:     entityID,
			})
		}

		// POST — upload attachment
		if cfg.UploadFile == nil || cfg.CreateAttachment == nil {
			log.Printf("Attachment upload deps not configured for %s", cfg.EntityType)
			return htmxError("attachment upload not configured")
		}

		err := viewCtx.Request.ParseMultipartForm(32 << 20)
		if err != nil {
			log.Printf("Failed to parse multipart form: %v", err)
			return htmxError("failed to parse upload")
		}

		description := viewCtx.Request.FormValue("description")

		fh, header, err := viewCtx.Request.FormFile("attachment_file")
		if err != nil {
			log.Printf("Failed to get uploaded file: %v", err)
			return htmxError("no file provided")
		}
		defer fh.Close()

		maxBytes := cfg.maxBytes()
		if header.Size > maxBytes {
			return htmxError(fmt.Sprintf("file too large: %d bytes (max %d)", header.Size, maxBytes))
		}

		content, err := io.ReadAll(fh)
		if err != nil {
			log.Printf("Failed to read uploaded file: %v", err)
			return htmxError("failed to read file")
		}

		newID := cfg.newID()
		objectKey := fmt.Sprintf("attachments/%s/%s/%s-%s", cfg.EntityType, entityID, newID, header.Filename)
		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		bucket := cfg.BucketName
		if bucket == "" {
			bucket = "attachments"
		}

		err = cfg.UploadFile(ctx, bucket, objectKey, content, contentType)
		if err != nil {
			log.Printf("Failed to upload attachment: %v", err)
			return htmxError("failed to upload file")
		}

		att := &attachmentpb.Attachment{
			Id:               newID,
			ModuleKey:        cfg.EntityType,
			ForeignKey:       entityID,
			Name:             header.Filename,
			StorageContainer: &bucket,
			StorageKey:       &objectKey,
			ContentType:      &contentType,
			FileSizeBytes:    &header.Size,
			Status:           "active",
			Active:           true,
		}
		if description != "" {
			att.Description = &description
		}

		_, err = cfg.CreateAttachment(ctx, &attachmentpb.CreateAttachmentRequest{Data: att})
		if err != nil {
			log.Printf("Failed to create attachment record: %v", err)
			return htmxError("failed to save attachment")
		}

		redirectURL := cfg.RedirectURL
		if redirectURL == "" {
			redirectURL = viewCtx.Request.Header.Get("HX-Current-URL")
		}

		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Trigger":  `{"formSuccess":true}`,
				"HX-Redirect": redirectURL,
			},
		}
	})
}

// NewDeleteAction creates a POST handler to delete an attachment.
func NewDeleteAction(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		if cfg.DeleteAttachment == nil {
			return htmxError("attachment delete not configured")
		}

		attachmentID := viewCtx.Request.FormValue("attachment_id")
		if attachmentID == "" {
			return htmxError("attachment ID is required")
		}

		_, err := cfg.DeleteAttachment(ctx, &attachmentpb.DeleteAttachmentRequest{
			Data: &attachmentpb.Attachment{Id: attachmentID},
		})
		if err != nil {
			log.Printf("Failed to delete attachment %s: %v", attachmentID, err)
			return htmxError("failed to delete attachment")
		}

		redirectURL := cfg.RedirectURL
		if redirectURL == "" {
			redirectURL = viewCtx.Request.Header.Get("HX-Current-URL")
		}

		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Trigger":  `{"formSuccess":true}`,
				"HX-Redirect": redirectURL,
			},
		}
	})
}

// BuildTable creates a TableConfig for displaying attachments.
func BuildTable(attachments []*attachmentpb.Attachment, cfg *Config, entityID string) *types.TableConfig {
	l := cfg.Labels

	columns := []types.TableColumn{
		{Key: "name", Label: l.FileName, Sortable: true},
		{Key: "content_type", Label: l.FileType, Sortable: true, Width: "120px"},
		{Key: "file_size", Label: l.FileSize, Sortable: true, Width: "100px"},
		{Key: "description", Label: l.Description, Sortable: false},
	}

	rows := []types.TableRow{}
	for _, a := range attachments {
		sizeStr := formatFileSize(a.GetFileSizeBytes())

		rows = append(rows, types.TableRow{
			ID: a.GetId(),
			Cells: []types.TableCell{
				{Type: "text", Value: a.GetName()},
				{Type: "text", Value: a.GetContentType()},
				{Type: "text", Value: sizeStr},
				{Type: "text", Value: a.GetDescription()},
			},
			Actions: []types.TableAction{
				{
					Type:     "delete",
					Label:    l.Delete,
					Action:   "delete",
					URL:      route.ResolveURL(cfg.DeleteURL, "id", entityID),
					ItemName: a.GetName(),
				},
			},
		})
	}

	types.ApplyColumnStyles(columns, rows)

	return &types.TableConfig{
		ID:      "attachments-table",
		Columns: columns,
		Rows:    rows,
		Labels:  cfg.TableLabels,
		EmptyState: types.TableEmptyState{
			Title:   l.EmptyTitle,
			Message: l.EmptyMessage,
		},
	}
}

// formatFileSize converts bytes to a human-readable string.
func formatFileSize(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffix := []string{"KB", "MB", "GB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), suffix[exp])
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
