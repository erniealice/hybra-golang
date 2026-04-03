package attachment

import (
	"testing"

	attachmentpb "github.com/erniealice/esqyma/pkg/schema/v1/domain/document/attachment"
)

// ---------------------------------------------------------------------------
// formatFileSize
// ---------------------------------------------------------------------------

func TestFormatFileSize_Zero(t *testing.T) {
	got := formatFileSize(0)
	if got != "0 B" {
		t.Errorf("expected '0 B', got %q", got)
	}
}

func TestFormatFileSize_Bytes(t *testing.T) {
	got := formatFileSize(512)
	if got != "512 B" {
		t.Errorf("expected '512 B', got %q", got)
	}
}

func TestFormatFileSize_OneByte(t *testing.T) {
	got := formatFileSize(1)
	if got != "1 B" {
		t.Errorf("expected '1 B', got %q", got)
	}
}

func TestFormatFileSize_BoundaryAt1024(t *testing.T) {
	// Exactly 1024 bytes = 1.0 KB
	got := formatFileSize(1024)
	if got != "1.0 KB" {
		t.Errorf("expected '1.0 KB', got %q", got)
	}
}

func TestFormatFileSize_JustBelow1024(t *testing.T) {
	got := formatFileSize(1023)
	if got != "1023 B" {
		t.Errorf("expected '1023 B', got %q", got)
	}
}

func TestFormatFileSize_KB(t *testing.T) {
	// 5120 bytes = 5.0 KB
	got := formatFileSize(5120)
	if got != "5.0 KB" {
		t.Errorf("expected '5.0 KB', got %q", got)
	}
}

func TestFormatFileSize_MB(t *testing.T) {
	// 2 * 1024 * 1024 = 2097152 bytes = 2.0 MB
	got := formatFileSize(2 * 1024 * 1024)
	if got != "2.0 MB" {
		t.Errorf("expected '2.0 MB', got %q", got)
	}
}

func TestFormatFileSize_GB(t *testing.T) {
	// 3 * 1024 * 1024 * 1024 = 3.0 GB
	got := formatFileSize(3 * 1024 * 1024 * 1024)
	if got != "3.0 GB" {
		t.Errorf("expected '3.0 GB', got %q", got)
	}
}

func TestFormatFileSize_FractionalKB(t *testing.T) {
	// 1536 bytes = 1.5 KB
	got := formatFileSize(1536)
	if got != "1.5 KB" {
		t.Errorf("expected '1.5 KB', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// htmxError
// ---------------------------------------------------------------------------

func TestHtmxError_SetsHXTriggerHeader(t *testing.T) {
	result := htmxError("something went wrong")

	trigger, ok := result.Headers["HX-Trigger"]
	if !ok {
		t.Fatal("expected HX-Trigger header to be set")
	}
	expected := `{"formError":"something went wrong"}`
	if trigger != expected {
		t.Errorf("expected HX-Trigger %q, got %q", expected, trigger)
	}
}

func TestHtmxError_StatusCode200(t *testing.T) {
	result := htmxError("error")
	if result.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Config.maxBytes
// ---------------------------------------------------------------------------

func TestConfig_MaxBytesDefault(t *testing.T) {
	cfg := &Config{}
	got := cfg.maxBytes()
	want := int64(10 << 20) // 10 MB
	if got != want {
		t.Errorf("expected default maxBytes=%d (10 MB), got %d", want, got)
	}
}

func TestConfig_MaxBytesCustom(t *testing.T) {
	cfg := &Config{MaxUploadBytes: 5 << 20} // 5 MB
	got := cfg.maxBytes()
	want := int64(5 << 20)
	if got != want {
		t.Errorf("expected custom maxBytes=%d, got %d", want, got)
	}
}

func TestConfig_MaxBytesZeroUsesDefault(t *testing.T) {
	cfg := &Config{MaxUploadBytes: 0}
	got := cfg.maxBytes()
	if got != DefaultMaxUploadBytes {
		t.Errorf("expected DefaultMaxUploadBytes=%d, got %d", DefaultMaxUploadBytes, got)
	}
}

// ---------------------------------------------------------------------------
// BuildTable
// ---------------------------------------------------------------------------

func int64Ptr(v int64) *int64 { return &v }
func strPtr(s string) *string { return &s }

func TestBuildTable_WithAttachments(t *testing.T) {
	cfg := &Config{
		EntityType: "product",
		DeleteURL:  "/app/products/detail/{id}/attachments/delete",
		Labels:     DefaultLabels(),
	}

	attachments := []*attachmentpb.Attachment{
		{
			Id:            "att-1",
			Name:          "invoice.pdf",
			ContentType:   strPtr("application/pdf"),
			FileSizeBytes: int64Ptr(2 * 1024 * 1024), // 2 MB
			Description:   strPtr("Monthly invoice"),
		},
		{
			Id:            "att-2",
			Name:          "photo.jpg",
			ContentType:   strPtr("image/jpeg"),
			FileSizeBytes: int64Ptr(512 * 1024), // 512 KB
		},
	}

	table := BuildTable(attachments, cfg, "prod-123")

	if table.ID != "attachments-table" {
		t.Errorf("expected table ID 'attachments-table', got %q", table.ID)
	}
	if len(table.Columns) != 4 {
		t.Errorf("expected 4 columns, got %d", len(table.Columns))
	}
	if len(table.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(table.Rows))
	}

	// First row: invoice.pdf
	row0 := table.Rows[0]
	if row0.ID != "att-1" {
		t.Errorf("expected row ID 'att-1', got %q", row0.ID)
	}
	if row0.Cells[0].Value != "invoice.pdf" {
		t.Errorf("expected cell[0] 'invoice.pdf', got %q", row0.Cells[0].Value)
	}
	if row0.Cells[1].Value != "application/pdf" {
		t.Errorf("expected cell[1] 'application/pdf', got %q", row0.Cells[1].Value)
	}
	if row0.Cells[2].Value != "2.0 MB" {
		t.Errorf("expected cell[2] '2.0 MB', got %q", row0.Cells[2].Value)
	}
	if row0.Cells[3].Value != "Monthly invoice" {
		t.Errorf("expected cell[3] 'Monthly invoice', got %q", row0.Cells[3].Value)
	}

	// Actions: delete with resolved URL
	if len(row0.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(row0.Actions))
	}
	if row0.Actions[0].Type != "delete" {
		t.Errorf("expected action type 'delete', got %q", row0.Actions[0].Type)
	}
	if row0.Actions[0].URL != "/app/products/detail/prod-123/attachments/delete" {
		t.Errorf("expected resolved URL, got %q", row0.Actions[0].URL)
	}

	// Second row: photo.jpg
	row1 := table.Rows[1]
	if row1.Cells[0].Value != "photo.jpg" {
		t.Errorf("expected 'photo.jpg', got %q", row1.Cells[0].Value)
	}
	if row1.Cells[2].Value != "512.0 KB" {
		t.Errorf("expected '512.0 KB', got %q", row1.Cells[2].Value)
	}
}

func TestBuildTable_EmptyAttachments(t *testing.T) {
	cfg := &Config{
		EntityType: "product",
		Labels:     DefaultLabels(),
	}

	table := BuildTable([]*attachmentpb.Attachment{}, cfg, "prod-123")

	if len(table.Rows) != 0 {
		t.Errorf("expected 0 rows for empty attachments, got %d", len(table.Rows))
	}
	if table.EmptyState.Title != "No attachments" {
		t.Errorf("expected empty state title 'No attachments', got %q", table.EmptyState.Title)
	}
}

func TestBuildTable_ColumnLabelsFromConfig(t *testing.T) {
	labels := DefaultLabels()
	labels.FileName = "Custom File Name"
	labels.FileType = "Custom Type"

	cfg := &Config{
		EntityType: "product",
		Labels:     labels,
	}

	table := BuildTable([]*attachmentpb.Attachment{}, cfg, "prod-123")

	if table.Columns[0].Label != "Custom File Name" {
		t.Errorf("expected column label 'Custom File Name', got %q", table.Columns[0].Label)
	}
	if table.Columns[1].Label != "Custom Type" {
		t.Errorf("expected column label 'Custom Type', got %q", table.Columns[1].Label)
	}
}

func TestBuildTable_ZeroSizeAttachment(t *testing.T) {
	cfg := &Config{
		EntityType: "product",
		Labels:     DefaultLabels(),
	}

	attachments := []*attachmentpb.Attachment{
		{
			Id:            "att-zero",
			Name:          "empty.txt",
			ContentType:   strPtr("text/plain"),
			FileSizeBytes: int64Ptr(0),
		},
	}

	table := BuildTable(attachments, cfg, "prod-123")

	if len(table.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(table.Rows))
	}
	if table.Rows[0].Cells[2].Value != "0 B" {
		t.Errorf("expected '0 B' for zero-size file, got %q", table.Rows[0].Cells[2].Value)
	}
}
