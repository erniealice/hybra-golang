package form

// UploadFormData is the template data for the upload drawer form.
type UploadFormData struct {
	FormAction   string
	WorkspaceID   string // injected by C1: populated by ViewAdapter.injectWorkspaceID for action_workspace_guard
	Labels       Labels
	CommonLabels any
	MaxFileSize  int64
	EntityType   string
	EntityID     string
}

// Labels holds UI text for the attachment feature.
type Labels struct {
	TabLabel          string // "Attachments"
	UploadTitle       string // "Upload Attachment"
	FileName          string // "File Name"
	FileInput         string // "Select File"
	Description       string // form-field label, e.g. "Description (optional)"
	DescriptionColumn string // table column header, e.g. "Description". Falls back to Description when empty.
	FileType          string // "Type"
	FileSize          string // "Size"
	UploadedAt        string // "Uploaded"
	UploadedBy        string // "Uploaded By"
	EmptyTitle        string // "No attachments"
	EmptyMessage      string // "Upload files to attach them to this record."
	Delete            string // "Delete"
	Upload            string // "Upload"
	Preview           string // "Preview"
}

// DefaultLabels returns English defaults for quick prototyping.
func DefaultLabels() Labels {
	return Labels{
		TabLabel:          "Attachments",
		UploadTitle:       "Upload Attachment",
		FileName:          "File Name",
		FileInput:         "Select File",
		Description:       "Description (optional)",
		DescriptionColumn: "Description",
		FileType:          "Type",
		FileSize:          "Size",
		UploadedAt:        "Uploaded",
		UploadedBy:        "Uploaded By",
		EmptyTitle:        "No attachments",
		EmptyMessage:      "Upload files to attach them to this record.",
		Delete:            "Delete",
		Upload:            "Upload",
		Preview:           "Preview",
	}
}
