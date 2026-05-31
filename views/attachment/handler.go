package attachment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	attachmentpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/document/attachment"
	"github.com/erniealice/pyeza-golang/route"
	"github.com/erniealice/pyeza-golang/types"
	"github.com/erniealice/pyeza-golang/view"

	"github.com/erniealice/hybra-golang/views/attachment/form"
)

// DefaultMaxUploadBytes is the fallback when Config.MaxUploadBytes is 0 (10 MB).
const DefaultMaxUploadBytes int64 = 10 << 20

// urlPlaceholderRe matches `{name}` placeholders in URL patterns.
var urlPlaceholderRe = regexp.MustCompile(`\{(\w+)\}`)

// Config parameterizes the generic attachment handlers for a specific entity.
type Config struct {
	EntityType     string // e.g. "product", "client", "asset"
	BucketName     string // storage bucket (default: "attachments")
	MaxUploadBytes int64  // 0 = use DefaultMaxUploadBytes
	UploadURL      string // POST form action URL pattern (may contain {id} and other placeholders)
	DeleteURL      string // POST delete action URL pattern (may contain {id} and other placeholders)
	DownloadURL    string // GET preview/download URL pattern. Empty = preview action hidden.
	RedirectURL    string // page to redirect to after action
	Labels         form.Labels
	CommonLabels   any
	TableLabels    types.TableLabels

	// RefreshURL is the tab-action URL used by the table to reload the
	// attachments tab after an upload or delete. It may contain {name}
	// placeholders (e.g. {id}, {ppid}) which BuildTable resolves via urlPairs.
	// Example: route.ResolveURL(deps.Routes.TabActionURL, "id", entityID, "tab", "attachments")
	RefreshURL string

	// PrimaryIDPathParam names the URL placeholder whose value identifies the
	// attached entity in storage (used as foreign_key + storage path segment).
	// Default: "id". For nested mounts: variant uses "vid", stock-item uses "iid".
	// Must match the foreign_key passed to ListAttachments by the caller.
	PrimaryIDPathParam string

	// Policy optionally overrides the per-domain upload allow-list for THIS
	// surface. When the zero value, the handler resolves the policy from the
	// central DefaultRegistry by EntityType (module_key). An unconfigured
	// EntityType with no override is DEFAULT-DENY: every upload is rejected.
	// Q-ST-POLICY (LOCKED): central registry now, migrate into the use case in W4.
	Policy *Policy

	// NewID generates a unique identifier for new attachments.
	NewID func() string

	// Storage operations (injected from composition root).
	//
	// UploadFile / DownloadFile are the ORIGINAL buffered ([]byte) closures. Their
	// signatures are a PUBLIC contract consumed by ~37 downstream view/module Deps
	// structs (centymo/entydad/fayna/fycha) that assign their own []byte closure
	// straight through to this Config, so they MUST NOT change shape. Streaming is
	// added ADDITIVELY below.
	UploadFile   func(ctx context.Context, bucket, key string, content []byte, contentType string) error
	DownloadFile func(ctx context.Context, bucket, key string) ([]byte, error)

	// Q-ST-STREAM (LOCKED, B+C): OPTIONAL STREAM-tier closures, additive and
	// nil-by-default. The composition root wires them to espyna's
	// StreamingStorageProvider (UploadStream / DownloadStream + io.Copy) when the
	// active adapter advertises it, leaving them nil otherwise. When set, the
	// upload/download handlers PREFER them so hybra never buffers the whole object:
	// upload hands UploadFileStream the (already MaxBytesReader-bounded) multipart
	// file reader; download pumps the DownloadFileStream ReadCloser straight to the
	// http.ResponseWriter via io.Copy. When nil, the handlers fall back to the
	// buffered UploadFile/DownloadFile above so the contract stays non-breaking.
	//
	// UploadFileStream streams `body` (the bounded multipart file reader) of
	// declared `size` bytes to (bucket,key). The caller does NOT pre-read body.
	UploadFileStream func(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error
	// DownloadFileStream opens (bucket,key) and returns the object body as an
	// io.ReadCloser the CALLER MUST Close, alongside the byte length (-1 when the
	// backend does not report it). The body is streamed, never fully buffered.
	DownloadFileStream func(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error)

	// Data operations (backed by protobuf use cases)
	ListAttachments  func(ctx context.Context, moduleKey, foreignKey string) (*attachmentpb.ListAttachmentsResponse, error)
	CreateAttachment func(ctx context.Context, req *attachmentpb.CreateAttachmentRequest) (*attachmentpb.CreateAttachmentResponse, error)
	ReadAttachment   func(ctx context.Context, req *attachmentpb.ReadAttachmentRequest) (*attachmentpb.ReadAttachmentResponse, error)
	DeleteAttachment func(ctx context.Context, req *attachmentpb.DeleteAttachmentRequest) (*attachmentpb.DeleteAttachmentResponse, error)
}

// DefaultLabels returns English defaults for quick prototyping.
// Deprecated: Use form.DefaultLabels() instead.
func DefaultLabels() form.Labels {
	return form.DefaultLabels()
}

func (c *Config) maxBytes() int64 {
	// A per-domain Policy.MaxSize tightens (never loosens) the cap: take the
	// smaller of the Config cap and the policy cap when both are set.
	cap := c.MaxUploadBytes
	if cap <= 0 {
		cap = DefaultMaxUploadBytes
	}
	if p := c.policy(); p.MaxSize > 0 && p.MaxSize < cap {
		cap = p.MaxSize
	}
	return cap
}

// policy resolves the effective upload allow-list for this surface: an explicit
// Config.Policy override wins; otherwise the central DefaultRegistry is consulted
// by EntityType (module_key). An unknown EntityType with no override yields the
// zero Policy, which denies every upload (DEFAULT-DENY).
func (c *Config) policy() Policy {
	if c.Policy != nil {
		return *c.Policy
	}
	return PolicyFor(c.EntityType)
}

func (c *Config) newID() string {
	if c.NewID != nil {
		return c.NewID()
	}
	return fmt.Sprintf("att-%d", 0)
}

func (c *Config) primaryIDParam() string {
	if c.PrimaryIDPathParam != "" {
		return c.PrimaryIDPathParam
	}
	return "id"
}

// resolveURLPairs reads every `{name}` placeholder in urlPattern and pairs it
// with the request's PathValue, so URLs with multiple placeholders (e.g. variant
// `/{id}/variant/{vid}/...`) resolve correctly when handed to route.ResolveURL.
func resolveURLPairs(req *http.Request, urlPattern string) []string {
	matches := urlPlaceholderRe.FindAllStringSubmatch(urlPattern, -1)
	pairs := make([]string, 0, len(matches)*2)
	for _, m := range matches {
		pairs = append(pairs, m[1], req.PathValue(m[1]))
	}
	return pairs
}

// NewUploadAction creates a dual-purpose handler: GET = drawer form, POST = upload file.
func NewUploadAction(cfg *Config) view.View {
	return view.ViewFunc(func(ctx context.Context, viewCtx *view.ViewContext) view.ViewResult {
		entityID := viewCtx.Request.PathValue(cfg.primaryIDParam())
		uploadPairs := resolveURLPairs(viewCtx.Request, cfg.UploadURL)

		if viewCtx.Request.Method == http.MethodGet {
			return view.OK("attachment-upload-drawer-form", &form.UploadFormData{
				FormAction:   route.ResolveURL(cfg.UploadURL, uploadPairs...),
				Labels:       cfg.Labels,
				CommonLabels: nil,
				MaxFileSize:  cfg.maxBytes(),
				EntityType:   cfg.EntityType,
				EntityID:     entityID,
				AcceptTypes:  cfg.policy().allowedExtensionsCSV(),
			})
		}

		// POST — upload attachment
		if cfg.UploadFile == nil || cfg.CreateAttachment == nil {
			log.Printf("Attachment upload deps not configured for %s", cfg.EntityType)
			return htmxError("attachment upload not configured")
		}

		maxBytes := cfg.maxBytes()

		// ST-H2 (OOM): cap the request body BEFORE any buffering. A multipart
		// body is read whole into memory by ParseMultipartForm + io.ReadAll, so
		// a (possibly spoofed Content-Length / streamed-chunked) oversized upload
		// could exhaust process memory. http.MaxBytesReader makes the reader stop
		// and error at the limit — enforcement happens on the wire, not after the
		// fact. A nil ResponseWriter is allowed here (this handler returns a
		// view.View and never gets the raw writer); the cap still applies, only
		// the 413-connection-close hint is skipped. The +bodyOverhead slack lets
		// the multipart envelope (boundaries, the description field, headers) ride
		// alongside a file at the full MaxUploadBytes without false-positive 413s;
		// the precise per-file ceiling is still re-checked via header.Size below.
		const bodyOverhead = 1 << 20 // 1 MiB for multipart framing + other fields
		viewCtx.Request.Body = http.MaxBytesReader(nil, viewCtx.Request.Body, maxBytes+bodyOverhead)

		err := viewCtx.Request.ParseMultipartForm(32 << 20)
		if err != nil {
			// A MaxBytesError here means the body blew the cap — surface a clean
			// "too large" rather than a generic parse failure.
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return htmxError(fmt.Sprintf("upload too large (max %d bytes)", maxBytes))
			}
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

		if header.Size > maxBytes {
			return htmxError(fmt.Sprintf("file too large: %d bytes (max %d)", header.Size, maxBytes))
		}

		policy := cfg.policy()

		// W4 (Q-ST-POLICY, MaxFileCount): reject when this parent record already
		// holds the policy's cap of active attachments. This is the CHEAP UX
		// rejection — counted from ListAttachments before any bytes are streamed.
		// The espyna CreateAttachment use case re-counts and rejects before insert
		// as the server-authoritative backstop for every caller; a benign TOCTOU
		// gap between this read and that insert is closed there. MaxFileCount == 0
		// means "no per-record cap" and skips the check. We fail OPEN on a list
		// error (the espyna gate still enforces) rather than block uploads on a
		// transient read failure.
		if policy.MaxFileCount > 0 && cfg.ListAttachments != nil {
			if listResp, lerr := cfg.ListAttachments(ctx, cfg.EntityType, entityID); lerr != nil {
				log.Printf("attachment count precheck: list %s/%s failed (failing open, espyna backstops): %v", cfg.EntityType, entityID, lerr)
			} else if n := countActive(listResp.GetData()); n >= policy.MaxFileCount {
				log.Printf("attachment upload rejected: module_key=%q foreign_key=%q active=%d cap=%d (MaxFileCount)", cfg.EntityType, entityID, n, policy.MaxFileCount)
				return htmxError(fmt.Sprintf("attachment limit reached (max %d files)", policy.MaxFileCount))
			}
		}

		// ST-H3 (file-type enforcement): the allow/deny decision is made on the
		// SERVER-DERIVED content type (magic-byte sniff of the first 512 bytes),
		// never on the attacker-controlled Content-Type header. The client header
		// is logged for forensics but is not trusted. An unconfigured module_key
		// resolves to the zero Policy, which denies everything (DEFAULT-DENY).
		//
		// Q-ST-STREAM: we read ONLY the sniff prefix (≤512 bytes) into memory, not
		// the whole file. The full payload is reconstituted as a stream via
		// io.MultiReader(prefix, rest-of-fh) and handed straight to UploadFile, so
		// even a multi-hundred-MB upload never lands fully in RAM. fh is still
		// wrapped by the MaxBytesReader applied above, so the byte ceiling keeps
		// being enforced as the stream is consumed by the storage writer.
		sniffPrefix := make([]byte, 512)
		nPrefix, err := io.ReadFull(fh, sniffPrefix)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return htmxError(fmt.Sprintf("upload too large (max %d bytes)", maxBytes))
			}
			log.Printf("Failed to read upload prefix: %v", err)
			return htmxError("failed to read file")
		}
		sniffPrefix = sniffPrefix[:nPrefix]

		sniffedType := sniffContentType(sniffPrefix)
		ext := extensionOf(header.Filename)
		clientType := header.Header.Get("Content-Type")
		if !policy.allows(sniffedType, ext) {
			log.Printf(
				"attachment upload rejected: module_key=%q filename=%q ext=%q sniffed=%q client_hdr=%q (policy denied)",
				cfg.EntityType, header.Filename, ext, sniffedType, clientType,
			)
			if allowed := policy.allowedExtensionsCSV(); allowed != "" {
				return htmxError(fmt.Sprintf("file type not allowed (accepted: %s)", allowed))
			}
			return htmxError("file uploads are not enabled for this record type")
		}

		newID := cfg.newID()
		// Sanitize the client filename before composing the storage key so a
		// crafted name cannot inject into the object-key namespace on cloud
		// providers (ST-M3). The DB `name` column keeps the original filename.
		safeName := sanitizeObjectKeySegment(header.Filename)
		// Q-ST-WSSCOPE (keyPrefix, LOCKED): this key is PROVISIONAL and workspace-
		// AGNOSTIC by design. hybra cannot read the trusted workspace_id from ctx
		// without taking an espyna module dependency (a layering violation), so the
		// authoritative workspace-prefix rewrite (and the WorkspaceId stamp on the
		// DB row) happens server-side in the espyna CreateAttachment use case from
		// the same ctx workspace value — keeping the object name and the row's
		// workspace_id derived from one server-trusted source at the create boundary.
		objectKey := fmt.Sprintf("attachments/%s/%s/%s-%s", cfg.EntityType, entityID, newID, safeName)
		// Store the server-derived content type, not the client header (ST-H3).
		contentType := normalizeContentType(sniffedType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		bucket := cfg.BucketName
		if bucket == "" {
			bucket = "attachments"
		}

		// Reconstitute the full payload: the already-consumed sniff prefix is
		// re-prepended to the remaining file reader so no bytes are lost.
		body := io.MultiReader(bytes.NewReader(sniffPrefix), fh)

		// PREFER the stream closure when the composition root opted in: it pumps
		// `body` through io.Copy into the backend's native streaming writer
		// (espyna StreamingStorageProvider), so even a multi-hundred-MB upload
		// never lands fully in RAM (the bounded-memory win). When the stream
		// closure is nil, FALL BACK to the original buffered []byte UploadFile:
		// io.ReadAll the (still MaxBytesReader-bounded) body and hand it over —
		// keeping the unchanged public contract working for every caller that has
		// not opted into streaming.
		if cfg.UploadFileStream != nil {
			if err = cfg.UploadFileStream(ctx, bucket, objectKey, body, header.Size, contentType); err != nil {
				var maxErr *http.MaxBytesError
				if errors.As(err, &maxErr) {
					return htmxError(fmt.Sprintf("upload too large (max %d bytes)", maxBytes))
				}
				log.Printf("Failed to upload attachment: %v", err)
				return htmxError("failed to upload file")
			}
		} else {
			content, rerr := io.ReadAll(body)
			if rerr != nil {
				var maxErr *http.MaxBytesError
				if errors.As(rerr, &maxErr) {
					return htmxError(fmt.Sprintf("upload too large (max %d bytes)", maxBytes))
				}
				log.Printf("Failed to read uploaded file: %v", rerr)
				return htmxError("failed to read file")
			}
			if err = cfg.UploadFile(ctx, bucket, objectKey, content, contentType); err != nil {
				log.Printf("Failed to upload attachment: %v", err)
				return htmxError("failed to upload file")
			}
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

		// Standard refresh pattern (matches centymo.HTMXSuccess). The drawer
		// closes via formSuccess and the table refreshes via refreshTable.
		// Previously returned HX-Redirect, which raced with the formSuccess
		// JS handler and surfaced as a spurious error in the drawer despite
		// a 200 OK and a saved row.
		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Trigger": `{"formSuccess":true,"refreshTable":"attachments-table"}`,
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

		// pyeza's table-actions.js posts delete with the attachment ID in the
		// `?id=` query string (the path's {id} is the parent entity). Fall
		// back to the form body for non-pyeza callers that POST it there.
		attachmentID := viewCtx.Request.URL.Query().Get("id")
		if attachmentID == "" {
			attachmentID = viewCtx.Request.FormValue("attachment_id")
		}
		if attachmentID == "" {
			return htmxError("attachment ID is required")
		}

		// ST-H1: the parent {id} path value is load-bearing — bind it as the
		// attachment's foreign_key and the route's entity type as its module_key
		// so the entity-scoped delete use case can assert ownership before any
		// destructive call. The DeleteAttachment closure is wired in composition
		// to DeleteAttachmentByEntity, which reads these off req.Data.
		foreignKey := viewCtx.Request.PathValue(cfg.primaryIDParam())

		_, err := cfg.DeleteAttachment(ctx, &attachmentpb.DeleteAttachmentRequest{
			Data: &attachmentpb.Attachment{
				Id:         attachmentID,
				ModuleKey:  cfg.EntityType,
				ForeignKey: foreignKey,
			},
		})
		if err != nil {
			log.Printf("Failed to delete attachment %s: %v", attachmentID, err)
			return htmxError("failed to delete attachment")
		}

		return view.ViewResult{
			StatusCode: http.StatusOK,
			Headers: map[string]string{
				"HX-Trigger": `{"formSuccess":true,"refreshTable":"attachments-table"}`,
			},
		}
	})
}

// NewDownloadHandler returns an http.HandlerFunc that streams an attachment's
// stored bytes inline (so browsers preview supported types in a new tab).
//
// The attachment ID is read from the `id` query parameter (set by pyeza's
// table-actions.js when a `download` action is clicked). The path's `{id}`
// placeholder identifies the parent entity and is used only for permission
// scoping by the surrounding route — not for the storage lookup.
func NewDownloadHandler(cfg *Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if cfg.ReadAttachment == nil || cfg.DownloadFile == nil {
			http.Error(w, "attachment download not configured", http.StatusServiceUnavailable)
			return
		}

		attachmentID := r.URL.Query().Get("id")
		if attachmentID == "" {
			http.Error(w, "attachment id is required", http.StatusBadRequest)
			return
		}

		// ST-H1: bind the parent {id} path value as the attachment's foreign_key
		// and the route's entity type as its module_key. The ReadAttachment
		// closure is wired in composition to ReadAttachmentByEntity, which fetches
		// by id and then asserts (module_key, foreign_key) + workspace ownership
		// on the metadata row BEFORE any byte stream is opened, returning an empty
		// (not-found-shaped) response on mismatch so existence is not leaked.
		foreignKey := r.PathValue(cfg.primaryIDParam())

		readResp, err := cfg.ReadAttachment(r.Context(), &attachmentpb.ReadAttachmentRequest{
			Data: &attachmentpb.Attachment{
				Id:         attachmentID,
				ModuleKey:  cfg.EntityType,
				ForeignKey: foreignKey,
			},
		})
		if err != nil {
			log.Printf("attachment download: read %s failed: %v", attachmentID, err)
			http.Error(w, "attachment not found", http.StatusNotFound)
			return
		}
		if len(readResp.GetData()) == 0 {
			http.Error(w, "attachment not found", http.StatusNotFound)
			return
		}
		att := readResp.GetData()[0]

		container := att.GetStorageContainer()
		key := att.GetStorageKey()
		if container == "" || key == "" {
			log.Printf("attachment download: %s missing storage_container/storage_key", attachmentID)
			http.Error(w, "attachment storage location missing", http.StatusInternalServerError)
			return
		}

		// PREFER the stream closure when the composition root opted in: open the
		// object as a stream and pump it to the client via io.Copy so the body
		// never lands fully in process memory (the bounded-memory win). When the
		// stream closure is nil, FALL BACK to the original buffered []byte
		// DownloadFile and wrap the bytes in a NopCloser so the downstream
		// io.Copy path is identical. DownloadFileStream returns an io.ReadCloser
		// the caller MUST Close (espyna DownloadStream / backend reader); size is
		// -1 when the backend cannot report a length.
		var (
			rc   io.ReadCloser
			size int64
		)
		if cfg.DownloadFileStream != nil {
			rc, size, err = cfg.DownloadFileStream(r.Context(), container, key)
		} else {
			var content []byte
			content, err = cfg.DownloadFile(r.Context(), container, key)
			if err == nil {
				rc = io.NopCloser(bytes.NewReader(content))
				size = int64(len(content))
			}
		}
		if err != nil {
			log.Printf("attachment download: storage fetch %s/%s failed: %v", container, key, err)
			http.Error(w, "failed to fetch attachment", http.StatusInternalServerError)
			return
		}
		defer rc.Close()

		contentType := att.GetContentType()
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// ST-H3: never let the browser MIME-sniff an uploader-controlled body, and
		// only allow inline rendering for a confirmed-safe content-type set
		// (images + PDF). Everything else — including text/html, image/svg+xml,
		// and application/octet-stream — is forced to download as an attachment so
		// a malicious upload cannot execute as stored XSS in the victim's session.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		disposition := "attachment"
		if isInlineSafeContentType(contentType) {
			disposition = "inline"
		}
		w.Header().Set("Content-Type", contentType)
		// Prefer the storage-reported length; fall back to the persisted metadata
		// size so the browser still shows a progress bar when the backend can't
		// report it inline. Omit the header entirely if neither is known (chunked).
		if size < 0 {
			size = att.GetFileSizeBytes()
		}
		if size > 0 {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, sanitizeHeaderFilename(att.GetName())))
		if _, err := io.Copy(w, rc); err != nil {
			// Headers (incl. 200) are already committed once Copy starts writing,
			// so we can't send an error status — just log the truncated transfer.
			log.Printf("attachment download: stream copy %s/%s failed: %v", container, key, err)
		}
	}
}

// inlineSafeContentTypes is the allowlist of content types the browser may render
// inline. SVG is intentionally excluded (it can carry script); HTML is excluded
// for the same reason. Everything not listed is served as an attachment.
var inlineSafeContentTypes = map[string]bool{
	"image/png":       true,
	"image/jpeg":      true,
	"image/gif":       true,
	"image/webp":      true,
	"image/bmp":       true,
	"application/pdf": true,
	"text/plain":      true,
}

// isInlineSafeContentType reports whether ct (ignoring any "; charset=" suffix and
// case) is on the inline allowlist.
func isInlineSafeContentType(ct string) bool {
	base := ct
	if i := strings.IndexByte(base, ';'); i >= 0 {
		base = base[:i]
	}
	base = strings.ToLower(strings.TrimSpace(base))
	return inlineSafeContentTypes[base]
}

// sanitizeHeaderFilename strips characters that could break out of the quoted
// Content-Disposition filename token (CR/LF, double-quote, backslash).
func sanitizeHeaderFilename(name string) string {
	return headerFilenameUnsafeRe.ReplaceAllString(name, "_")
}

var headerFilenameUnsafeRe = regexp.MustCompile(`[\r\n"\\]`)

// sanitizeObjectKeySegment makes a client filename safe to concatenate into a
// storage object key on any provider (S3/GCS/Azure do not sanitize like the
// local adapter does, ST-M3). It drops any path component, replaces path
// separators / "..", control chars and other key-hostile bytes with "_", and
// caps the length. The result never contains "/", "\", or ".." so it cannot
// traverse the key namespace.
func sanitizeObjectKeySegment(name string) string {
	// Keep only the base name — discard any directory components a crafted
	// "../../etc/passwd" filename might carry.
	name = name[strings.LastIndexAny(name, `/\`)+1:]
	name = objectKeyUnsafeRe.ReplaceAllString(name, "_")
	name = strings.ReplaceAll(name, "..", "__")
	name = strings.Trim(name, ". ")
	if name == "" {
		name = "file"
	}
	if len(name) > 128 {
		name = name[:128]
	}
	return name
}

// objectKeyUnsafeRe matches bytes that must not appear in a storage object key
// segment: path separators, control chars, and shell/URL-hostile characters.
var objectKeyUnsafeRe = regexp.MustCompile(`[^\w.\-]+`)

// BuildTable creates a TableConfig for displaying attachments.
//
// entityID maps to the {id} placeholder in cfg.UploadURL and cfg.DeleteURL.
// For URLs with additional placeholders (e.g. variant: {id}+{vid}, stock-item:
// {id}+{vid}+{iid}), pass extra pairs via extraURLPairs ("vid", vidValue, ...).
func BuildTable(attachments []*attachmentpb.Attachment, cfg *Config, entityID string, extraURLPairs ...string) *types.TableConfig {
	l := cfg.Labels

	urlPairs := append([]string{"id", entityID}, extraURLPairs...)

	descriptionColumnLabel := l.DescriptionColumn
	if descriptionColumnLabel == "" {
		descriptionColumnLabel = l.Description
	}

	columns := []types.TableColumn{
		{Key: "name", Label: l.FileName},
		{Key: "content_type", Label: l.FileType, WidthClass: "col-2xl"},
		{Key: "file_size", Label: l.FileSize, WidthClass: "col-lg"},
		{Key: "description", Label: descriptionColumnLabel, NoSort: true},
	}

	previewURL := ""
	if cfg.DownloadURL != "" {
		previewURL = route.ResolveURL(cfg.DownloadURL, urlPairs...)
	}

	rows := []types.TableRow{}
	for _, a := range attachments {
		sizeStr := formatFileSize(a.GetFileSizeBytes())

		actions := []types.TableAction{}
		if previewURL != "" {
			actions = append(actions, types.TableAction{
				Type:   "preview",
				Label:  l.Preview,
				Action: "download",
				URL:    previewURL,
			})
		}
		actions = append(actions, types.TableAction{
			Type:     "delete",
			Label:    l.Delete,
			Action:   "delete",
			URL:      route.ResolveURL(cfg.DeleteURL, urlPairs...),
			ItemName: a.GetName(),
		})

		rows = append(rows, types.TableRow{
			ID: a.GetId(),
			Cells: []types.TableCell{
				{Type: "text", Value: a.GetName()},
				{Type: "text", Value: a.GetContentType()},
				{Type: "text", Value: sizeStr},
				{Type: "text", Value: a.GetDescription()},
			},
			Actions: actions,
		})
	}

	types.ApplyColumnStyles(columns, rows)

	refreshURL := ""
	if cfg.RefreshURL != "" {
		// Append "tab","attachments" so TabActionURL templates (which carry a
		// {tab} placeholder) resolve to the correct tab without callers having
		// to hard-code the slug.  Extra pairs that don't match any placeholder
		// are silently ignored by route.ResolveURL.
		refreshPairs := append(urlPairs, "tab", "attachments")
		refreshURL = route.ResolveURL(cfg.RefreshURL, refreshPairs...)
	}

	table := &types.TableConfig{
		ID:                   "attachments-table",
		RefreshURL:           refreshURL,
		Columns:              columns,
		Rows:                 rows,
		ShowActions:          true,
		ShowSearch:           true,
		ShowSort:             true,
		ShowColumns:          true,
		ShowDensity:          true,
		ShowEntries:          true,
		DefaultSortColumn:    "name",
		DefaultSortDirection: "asc",
		Labels:               cfg.TableLabels,
		EmptyState: types.TableEmptyState{
			Title:   l.EmptyTitle,
			Message: l.EmptyMessage,
		},
	}

	if cfg.UploadURL != "" {
		table.PrimaryAction = &types.PrimaryAction{
			Label:     l.Upload,
			ActionURL: route.ResolveURL(cfg.UploadURL, urlPairs...),
			TestID:    "attachment-upload",
		}
	}

	types.ApplyTableSettings(table)

	return table
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

// countActive returns the number of attachments that count against a parent
// record's MaxFileCount cap: rows that are still Active (a soft-deleted row is
// Active=false and must not consume a slot). Pairs with the espyna use-case
// backstop, which counts the same active set authoritatively before insert.
func countActive(atts []*attachmentpb.Attachment) int {
	n := 0
	for _, a := range atts {
		if a.GetActive() {
			n++
		}
	}
	return n
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
