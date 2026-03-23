package attachment

import (
	"context"

	attachmentpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/document/attachment"
)

// AttachmentOps bundles the cross-cutting function dependencies for attachment
// operations. Embed this struct in view-level and module-level Deps structs
// to avoid repeating 5 individual fields.
//
// Go struct embedding promotes these fields, so deps.UploadFile,
// deps.ListAttachments, etc. continue to resolve at call sites.
//
// Only embed in views that actually use attachment functionality.
// Views without attachments (e.g., job_activity) should NOT embed this.
type AttachmentOps struct {
	UploadFile       func(ctx context.Context, bucket, key string, content []byte, contentType string) error
	ListAttachments  func(ctx context.Context, moduleKey, foreignKey string) (*attachmentpb.ListAttachmentsResponse, error)
	CreateAttachment func(ctx context.Context, req *attachmentpb.CreateAttachmentRequest) (*attachmentpb.CreateAttachmentResponse, error)
	DeleteAttachment func(ctx context.Context, req *attachmentpb.DeleteAttachmentRequest) (*attachmentpb.DeleteAttachmentResponse, error)
	NewAttachmentID  func() string
}
