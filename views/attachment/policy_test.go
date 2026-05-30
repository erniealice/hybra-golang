package attachment

import (
	"bytes"
	"testing"
)

// pdfBytes / pngBytes / htmlBytes are minimal magic-byte prefixes that
// http.DetectContentType recognizes.
var (
	pdfBytes  = []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
	pngBytes  = append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0}, 16)...)
	htmlBytes = []byte("<!DOCTYPE html><html><script>alert(1)</script></html>")
	gifBytes  = []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00")
)

// ---------------------------------------------------------------------------
// DEFAULT-DENY
// ---------------------------------------------------------------------------

func TestPolicyFor_UnknownModuleKeyIsDefaultDeny(t *testing.T) {
	p := PolicyFor("totally-unknown-module")
	if len(p.AllowedContentTypes) != 0 || len(p.AllowedExtensions) != 0 {
		t.Fatalf("expected zero (deny) policy for unknown module, got %+v", p)
	}
	// A zero policy must reject everything, including otherwise-safe types.
	if p.allows("application/pdf", ".pdf") {
		t.Error("default-deny policy must reject a PDF")
	}
	if p.allows("image/png", ".png") {
		t.Error("default-deny policy must reject a PNG")
	}
}

func TestConfig_PolicyDefaultsToDenyForUnknownEntityType(t *testing.T) {
	cfg := &Config{EntityType: "no-such-entity"}
	if cfg.policy().allows("application/pdf", ".pdf") {
		t.Error("unconfigured EntityType with no override must default-deny")
	}
}

// ---------------------------------------------------------------------------
// Sniff-based allow/deny (the client header is NOT the basis for the decision)
// ---------------------------------------------------------------------------

func TestPolicy_AllowsSniffedPDFForCommonSafeModule(t *testing.T) {
	p := PolicyFor("revenue") // commonSafe
	sniffed := sniffContentType(pdfBytes)
	if !p.allows(sniffed, ".pdf") {
		t.Errorf("revenue policy should allow a sniffed PDF (sniffed=%q)", sniffed)
	}
}

func TestPolicy_RejectsHTMLEvenIfClientClaimsPNG(t *testing.T) {
	// Simulate the attack: a .html body with a spoofed image extension/header.
	// The decision must be made on the sniffed type, which is text/html.
	p := PolicyFor("revenue")
	sniffed := sniffContentType(htmlBytes)
	if sniffed != "text/html; charset=utf-8" {
		t.Fatalf("precondition: expected html sniff, got %q", sniffed)
	}
	// Even with a friendly extension, html content must be rejected.
	if p.allows(sniffed, ".png") {
		t.Error("commonSafe policy must reject sniffed text/html (stored-XSS vector)")
	}
}

func TestPolicy_RejectsTypeAllowedButExtensionMismatch(t *testing.T) {
	// Sniffed type is fine (PDF) but the extension is not on the allow-list.
	p := PolicyFor("revenue")
	sniffed := sniffContentType(pdfBytes)
	if p.allows(sniffed, ".exe") {
		t.Error("policy must reject when extension is not allow-listed even if type is")
	}
}

func TestPolicy_ImagesOnlyRejectsPDF(t *testing.T) {
	p := PolicyFor("product") // imagesOnly
	if p.allows(sniffContentType(pdfBytes), ".pdf") {
		t.Error("product (images-only) policy must reject a PDF")
	}
	if !p.allows(sniffContentType(pngBytes), ".png") {
		t.Error("product (images-only) policy must allow a PNG")
	}
	if !p.allows(sniffContentType(gifBytes), ".gif") {
		t.Error("product (images-only) policy must allow a GIF")
	}
}

// ---------------------------------------------------------------------------
// Config.Policy override
// ---------------------------------------------------------------------------

func TestConfig_PolicyOverrideWins(t *testing.T) {
	override := &Policy{
		AllowedContentTypes: []string{"application/pdf"},
		AllowedExtensions:   []string{".pdf"},
	}
	cfg := &Config{EntityType: "product", Policy: override} // product would be images-only
	if !cfg.policy().allows(sniffContentType(pdfBytes), ".pdf") {
		t.Error("explicit Config.Policy override should allow a PDF even though product is images-only")
	}
	if cfg.policy().allows(sniffContentType(pngBytes), ".png") {
		t.Error("explicit override (pdf-only) should reject a PNG")
	}
}

// ---------------------------------------------------------------------------
// MaxSize tightening
// ---------------------------------------------------------------------------

func TestConfig_PolicyMaxSizeTightensCap(t *testing.T) {
	cfg := &Config{
		EntityType:     "revenue",
		MaxUploadBytes: 10 << 20,
		Policy:         &Policy{AllowedContentTypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}, MaxSize: 2 << 20},
	}
	if got, want := cfg.maxBytes(), int64(2<<20); got != want {
		t.Errorf("policy MaxSize should tighten cap to %d, got %d", want, got)
	}
}

func TestConfig_PolicyMaxSizeDoesNotLoosenCap(t *testing.T) {
	cfg := &Config{
		EntityType:     "revenue",
		MaxUploadBytes: 5 << 20,
		Policy:         &Policy{AllowedContentTypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}, MaxSize: 50 << 20},
	}
	if got, want := cfg.maxBytes(), int64(5<<20); got != want {
		t.Errorf("policy MaxSize larger than config must not loosen cap; want %d got %d", want, got)
	}
}

// ---------------------------------------------------------------------------
// Object-key segment sanitization (ST-M3)
// ---------------------------------------------------------------------------

func TestSanitizeObjectKeySegment_StripsTraversal(t *testing.T) {
	cases := map[string]func(string) bool{
		"../../etc/passwd":   func(s string) bool { return s == "passwd" },
		"a/b/c.png":          func(s string) bool { return s == "c.png" },
		`a\b\c.png`:          func(s string) bool { return s == "c.png" },
		"normal-file_1.pdf":  func(s string) bool { return s == "normal-file_1.pdf" },
		"weird name!@#$.jpg": func(s string) bool { return s != "" && !containsAny(s, `/\`) },
		"":                   func(s string) bool { return s == "file" },
	}
	for in, ok := range cases {
		got := sanitizeObjectKeySegment(in)
		if !ok(got) {
			t.Errorf("sanitizeObjectKeySegment(%q) = %q (failed invariant)", in, got)
		}
		if containsAny(got, `/\`) || got == ".." {
			t.Errorf("sanitizeObjectKeySegment(%q) = %q still key-hostile", in, got)
		}
	}
}

func containsAny(s, chars string) bool {
	for _, c := range chars {
		for _, r := range s {
			if r == c {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Registry coverage sanity: every known module key resolves to a non-deny policy
// ---------------------------------------------------------------------------

func TestRegistry_AllRegisteredKeysAreAllowSomething(t *testing.T) {
	for _, k := range RegisteredModuleKeys() {
		p := PolicyFor(k)
		if len(p.AllowedContentTypes) == 0 || len(p.AllowedExtensions) == 0 {
			t.Errorf("registered module %q has an empty (deny) policy — registry entries must allow at least one type", k)
		}
	}
}
