package form

import "github.com/erniealice/pyeza-golang/types"

// UploadFormData is the template data for the upload drawer form.
type UploadFormData struct {
	FormAction   string
	Labels       Labels
	CommonLabels any
	AcceptTypes  string // e.g. ".docx" — for the file input accept attribute
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
