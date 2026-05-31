package attachment

import (
	"context"
	"io"

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
	// UploadFile / DownloadFile are the ORIGINAL buffered ([]byte) storage
	// closures — these MUST stay []byte-shaped: ~37 downstream view/module Deps
	// structs across centymo/entydad/fayna/fycha declare a field of this exact
	// signature and assign it straight through to attachment.Config. Changing the
	// shape here is a breaking public-contract change that fails the whole-
	// workspace build, so streaming is added ADDITIVELY below (mirroring espyna's
	// StreamingStorageProvider sub-interface) rather than by mutating these.
	UploadFile   func(ctx context.Context, bucket, key string, content []byte, contentType string) error
	DownloadFile func(ctx context.Context, bucket, key string) ([]byte, error)

	// Q-ST-STREAM (LOCKED, B+C): OPTIONAL STREAM-tier closures, additive and
	// nil-by-default. When a caller opts in (the composition root wires these to
	// espyna's StreamingStorageProvider via UploadStream/DownloadStream + io.Copy),
	// the handler PREFERS them for bounded-memory transfer; when nil it falls back
	// to the buffered UploadFile/DownloadFile above. UploadFileStream streams a
	// bounded io.Reader of `size` bytes; DownloadFileStream returns an
	// io.ReadCloser the caller MUST Close + the byte length (-1 when unknown).
	// Downstream view/module Deps need NOT declare these — only the real app
	// composition root opts in.
	UploadFileStream   func(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error
	DownloadFileStream func(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error)

	ListAttachments  func(ctx context.Context, moduleKey, foreignKey string) (*attachmentpb.ListAttachmentsResponse, error)
	CreateAttachment func(ctx context.Context, req *attachmentpb.CreateAttachmentRequest) (*attachmentpb.CreateAttachmentResponse, error)
	ReadAttachment   func(ctx context.Context, req *attachmentpb.ReadAttachmentRequest) (*attachmentpb.ReadAttachmentResponse, error)
	DeleteAttachment func(ctx context.Context, req *attachmentpb.DeleteAttachmentRequest) (*attachmentpb.DeleteAttachmentResponse, error)
	NewAttachmentID  func() string
}
