package attachment

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

// Policy is the per-domain (module_key) upload allow-list. It governs what kinds
// of files a given attachment surface will accept. Validation is performed in
// the upload path against the SERVER-DERIVED content type (magic-byte sniff via
// http.DetectContentType) — the client-supplied Content-Type header is never the
// basis for the allow/deny decision (ST-H3).
//
// Q-ST-POLICY (LOCKED): a central default-deny registry lives here now; this
// migrates into the attachment use case in W4. Until then the hybra upload
// handler consults DefaultRegistry by module_key.
type Policy struct {
	// AllowedContentTypes is the set of sniffed MIME types this domain accepts.
	// Compared (case-insensitively, ignoring any "; charset=" suffix) against the
	// http.DetectContentType result on the first 512 bytes. Empty => deny all.
	AllowedContentTypes []string

	// AllowedExtensions is the set of accepted filename extensions, each with a
	// leading dot and lower-cased (e.g. ".pdf", ".png"). The uploaded filename's
	// extension must be a member. Empty => deny all.
	AllowedExtensions []string

	// MaxFileCount is the maximum number of attachments allowed on a single
	// parent record. 0 means "no per-record count limit enforced here" (the
	// upload handler treats 0 as unlimited; tightening is a W4 use-case concern).
	MaxFileCount int

	// MaxSize is the per-file byte ceiling for this domain. 0 falls back to the
	// Config's MaxUploadBytes / DefaultMaxUploadBytes. A non-zero MaxSize that is
	// smaller than the Config cap tightens it for this domain specifically.
	MaxSize int64
}

// allows reports whether the (sniffedType, ext) pair is permitted by this policy.
// Both checks must pass: the sniffed MIME type must be allow-listed AND the
// filename extension must be allow-listed. A policy with no allowed types or no
// allowed extensions denies everything (default-deny).
func (p Policy) allows(sniffedType, ext string) bool {
	if len(p.AllowedContentTypes) == 0 || len(p.AllowedExtensions) == 0 {
		return false
	}
	return p.allowsContentType(sniffedType) && p.allowsExtension(ext)
}

func (p Policy) allowsContentType(sniffedType string) bool {
	base := normalizeContentType(sniffedType)
	for _, ct := range p.AllowedContentTypes {
		if normalizeContentType(ct) == base {
			return true
		}
	}
	return false
}

func (p Policy) allowsExtension(ext string) bool {
	want := strings.ToLower(ext)
	for _, e := range p.AllowedExtensions {
		if strings.ToLower(e) == want {
			return true
		}
	}
	return false
}

// allowedExtensionsCSV renders the policy's extensions as a comma-joined string
// for the dropzone `accept=` attribute / error messaging. Returns "" when the
// policy denies everything.
func (p Policy) allowedExtensionsCSV() string {
	if len(p.AllowedExtensions) == 0 {
		return ""
	}
	return strings.Join(p.AllowedExtensions, ",")
}

// normalizeContentType lower-cases a MIME type and strips any parameters
// (e.g. "text/plain; charset=utf-8" -> "text/plain").
func normalizeContentType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return strings.ToLower(strings.TrimSpace(ct))
}

// ----------------------------------------------------------------------------
// Reusable policy building blocks
// ----------------------------------------------------------------------------

// commonSafeContentTypes is the conservative "documents + images" allow-list
// applied to most business-record attachment surfaces. It excludes executables,
// archives, HTML, and SVG (all of which carry active-content / stored-XSS risk).
//
// NOTE on http.DetectContentType limits: it sniffs from a fixed signature table.
// Modern Office files (.docx/.xlsx) are ZIP containers, so they sniff as
// "application/zip" (or "application/x-zip-compressed"); legacy .doc/.xls sniff
// as "application/x-ole-storage" or the CDF signature. Plain text and CSV sniff
// as "text/plain". These realities are encoded below so legitimate business
// uploads are not falsely rejected, while the extension check still pins intent.
var commonSafeContentTypes = []string{
	"application/pdf",
	"image/png",
	"image/jpeg",
	"image/gif",
	"image/webp",
	"image/bmp",
	"image/tiff",
	"text/plain",
	"text/csv",
	// Office containers as seen by http.DetectContentType:
	"application/zip",              // .docx/.xlsx/.pptx (OOXML are zip)
	"application/x-zip-compressed", // some platforms label zip this way
	"application/octet-stream",     // some OOXML / older formats sniff generic
	"application/x-ole-storage",    // legacy .doc/.xls (OLE compound file)
}

// commonSafeExtensions is the matching extension allow-list for the documents +
// images surface.
var commonSafeExtensions = []string{
	".pdf",
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff",
	".txt", ".csv",
	".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
}

// imagesOnlyContentTypes / imagesOnlyExtensions are a tighter surface for
// domains that should only carry pictures (e.g. product photos).
var imagesOnlyContentTypes = []string{
	"image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp", "image/tiff",
}

var imagesOnlyExtensions = []string{
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff",
}

// commonSafePolicy is the conservative default applied to known business-record
// module keys: documents + images, no executables/archives/HTML/SVG. Owning
// teams should tighten per module (see the TODO map below).
func commonSafePolicy() Policy {
	return Policy{
		AllowedContentTypes: commonSafeContentTypes,
		AllowedExtensions:   commonSafeExtensions,
		MaxFileCount:        0, // TODO(owning-team): cap per module in W4
		MaxSize:             0, // inherit Config cap
	}
}

// imagesOnlyPolicy restricts a surface to images.
func imagesOnlyPolicy() Policy {
	return Policy{
		AllowedContentTypes: imagesOnlyContentTypes,
		AllowedExtensions:   imagesOnlyExtensions,
		MaxFileCount:        0,
		MaxSize:             0,
	}
}

// ----------------------------------------------------------------------------
// Central per-domain registry (keyed by module_key == Config.EntityType)
// ----------------------------------------------------------------------------

// DefaultRegistry maps a module_key (the value each call site passes as
// Config.EntityType) to its upload Policy. The hybra upload handler consults
// this registry by Config.EntityType, so the ~40 existing attachment.Config{}
// call sites are covered centrally without per-site edits.
//
// DEFAULT-DENY: a module_key with no entry here is rejected — PolicyFor returns
// a zero Policy, whose allows() returns false for every (type, ext) pair. To
// onboard a new attachment surface you MUST add a row here (or set Config.Policy
// explicitly); silence means "no uploads accepted", never "allow all".
//
// The conservative seed below is commonSafe (documents + images) for every
// module_key that ships an attachment surface today, with product surfaces
// pinned to images-only. TODO(owning-team): tighten each row to the exact set
// the domain actually needs.
var DefaultRegistry = map[string]Policy{
	// --- centymo: commerce records (documents + images) ---
	"accrued_expense":                  commonSafePolicy(),
	"collection":                       commonSafePolicy(),
	"disbursement":                     commonSafePolicy(),
	"expenditure":                      commonSafePolicy(),
	"expense_recognition":              commonSafePolicy(),
	"inventory":                        commonSafePolicy(),
	"plan":                             commonSafePolicy(),
	"price_plan":                       commonSafePolicy(),
	"price_schedule":                   commonSafePolicy(),
	"pricelist":                        commonSafePolicy(),
	"procurement_request":              commonSafePolicy(),
	"purchase_order":                   commonSafePolicy(),
	"revenue":                          commonSafePolicy(),
	"revenue_run":                      commonSafePolicy(),
	"subscription":                     commonSafePolicy(),
	"supplier_contract":                commonSafePolicy(),
	"supplier_contract_price_schedule": commonSafePolicy(),
	"line":                             commonSafePolicy(),

	// product surfaces carry catalog photos -> images-only is the tighter,
	// safer default. TODO(catalog-team): allow .pdf spec sheets if needed.
	"product":    imagesOnlyPolicy(),
	"variant":    imagesOnlyPolicy(),
	"stock-item": imagesOnlyPolicy(),

	// --- entydad: identity / org records (documents + images) ---
	"client":         commonSafePolicy(),
	"supplier":       commonSafePolicy(),
	"user":           commonSafePolicy(),
	"workspace":      commonSafePolicy(),
	"workspace_user": commonSafePolicy(),
	"location":       commonSafePolicy(),
	"role":           commonSafePolicy(),

	// --- fayna: operations / jobs (documents + images) ---
	"fulfillment":      commonSafePolicy(),
	"job":              commonSafePolicy(),
	"job_activity":     commonSafePolicy(),
	"job_phase":        commonSafePolicy(),
	"job_task":         commonSafePolicy(),
	"job_template":     commonSafePolicy(),
	"outcome_criteria": commonSafePolicy(),
	"task_outcome":     commonSafePolicy(),

	// --- fycha: accounting / assets (documents + images) ---
	"asset":         commonSafePolicy(),
	"journal_entry": commonSafePolicy(),

	// --- cyta: scheduling (documents + images) ---
	"event": commonSafePolicy(),
}

// PolicyFor returns the effective Policy for a given module_key. Resolution
// order:
//  1. an explicit Config.Policy override (handled by the caller, not here);
//  2. the DefaultRegistry entry for moduleKey;
//  3. DEFAULT-DENY: a zero Policy (rejects every upload).
func PolicyFor(moduleKey string) Policy {
	if p, ok := DefaultRegistry[moduleKey]; ok {
		return p
	}
	return Policy{} // default-deny
}

// RegisteredModuleKeys returns the sorted set of module keys with an explicit
// policy. Useful for boot-time auditing / tests.
func RegisteredModuleKeys() []string {
	keys := make([]string, 0, len(DefaultRegistry))
	for k := range DefaultRegistry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// extensionOf returns the lower-cased extension (with leading dot) of a
// filename, or "" if there is none.
func extensionOf(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

// sniffContentType reads up to the first 512 bytes of buf and returns the
// magic-byte-derived MIME type via http.DetectContentType. The client-supplied
// Content-Type header is intentionally NOT consulted here.
func sniffContentType(buf []byte) string {
	if len(buf) > 512 {
		buf = buf[:512]
	}
	return http.DetectContentType(buf)
}
