# hybra-golang

**Cross-cutting application features for the Ryta OS monorepo.**

`hybra-golang` is a shared package that houses reusable application-level orchestration code — features that are needed by multiple domain packages but don't belong in the UI framework (`pyeza`) or any single domain.

## Why hybra exists

The Ryta OS monorepo has a three-layer architecture:

```
  Domain Packages          centymo (commerce), entydad (identity), fycha (accounting)
        │                  Business logic, entity-specific views, route wiring
        ▼
  Shared Features          hybra (this package)
        │                  Cross-cutting application patterns used by all domains
        ▼
  Framework Layer          pyeza (UI), espyna (backend), esqyma (proto schemas)
                           Presentation primitives, infrastructure, data contracts
```

### The problem hybra solves

Before hybra, the generic attachment handler lived in `fycha-golang` (the accounting domain). Commerce (`centymo`) and Identity (`entydad`) imported it — creating **lateral dependencies** between domain packages:

```
centymo ──→ fycha/views/attachment   ← WRONG: commerce depends on accounting
entydad ──→ fycha/views/attachment   ← WRONG: identity depends on accounting
```

This violates the rule that domain packages should only depend **downward** (on framework packages), never **sideways** (on peer domain packages).

### The solution

hybra provides a neutral home for cross-cutting features. All domains depend downward on hybra:

```
centymo ──→ hybra/views/attachment   ← CORRECT: downward dependency
entydad ──→ hybra/views/attachment   ← CORRECT: downward dependency
fycha   ──→ hybra/views/attachment   ← CORRECT: downward dependency
```

## What lives in hybra

hybra contains **reusable application-level handlers** — Go code that orchestrates HTTP requests, storage operations, and CRUD operations, parameterized via dependency injection.

### Current features

#### `views/attachment/` — Generic document attachment system

A fully parameterized attachment handler used by 14+ entities across 3 domain packages. Provides:

- **`Config`** struct — parameterizes the handler for any entity type (product, client, asset, etc.)
- **`NewUploadAction(cfg)`** — dual-purpose view: GET returns the upload drawer form, POST handles multipart file upload → storage → database
- **`NewDeleteAction(cfg)`** — POST handler to delete an attachment
- **`BuildTable(attachments, cfg, entityID)`** — builds a `types.TableConfig` for displaying attachments in a table
- **`Labels`** struct + **`DefaultLabels()`** — UI text for attachment features
- **`UploadFormData`** — template data for the upload drawer form

**Usage in a domain package:**

```go
// In centymo/views/product/detail/attachment.go
package detail

import (
    "github.com/erniealice/hybra-golang/views/attachment"
    "github.com/erniealice/pyeza-golang/view"
)

func attachmentConfig(deps *Deps) *attachment.Config {
    return &attachment.Config{
        EntityType:       "product",
        BucketName:       "attachments",
        UploadURL:        deps.Routes.AttachmentUploadURL,
        DeleteURL:        deps.Routes.AttachmentDeleteURL,
        Labels:           attachment.DefaultLabels(),
        CommonLabels:     deps.CommonLabels,
        TableLabels:      deps.TableLabels,
        NewID:            deps.NewID,
        UploadFile:       deps.UploadFile,
        ListAttachments:  deps.ListAttachments,
        CreateAttachment: deps.CreateAttachment,
        DeleteAttachment: deps.DeleteAttachment,
    }
}

func NewAttachmentUploadAction(deps *Deps) view.View {
    return attachment.NewUploadAction(attachmentConfig(deps))
}

func NewAttachmentDeleteAction(deps *Deps) view.View {
    return attachment.NewDeleteAction(attachmentConfig(deps))
}
```

### Planned features (future)

These features follow the same pattern — cross-cutting application orchestration used by multiple domains:

- **`views/comments/`** — Threaded comment system for any entity
- **`views/auditlog/`** — Audit trail viewer for entity changes
- **`views/tags/`** — Tagging/labeling system for any entity
- **`views/activity/`** — Activity feed for entity events
- **`views/notes/`** — Note/memo system for any entity

## What does NOT live in hybra

| Concern | Where it belongs | Why |
|---------|-----------------|-----|
| HTML templates (attachment-tab, upload form) | `pyeza/templates/blocks/` | Pure presentation — no Go logic |
| CSS/JS (file-upload.css, file-upload.js) | `pyeza/styles/`, `pyeza/assets/js/` | Client-side behavior — framework concern |
| Domain-specific upload workflows | Domain packages (e.g., centymo invoice template upload) | Business rules, not cross-cutting |
| Proto schemas (attachment.proto) | `esqyma/proto/v1/domain/document/attachment/` | Data contract — schema layer |
| Storage adapters (S3, GCS, local) | `espyna/` | Infrastructure — backend framework |
| Entity-specific Config wrappers | Domain packages (e.g., `centymo/views/product/detail/attachment.go`) | Thin wiring — domain decides entity type, bucket, routes |

## Dependencies

```
hybra-golang
  ├── pyeza-golang/view      (View interface, ViewFunc, ViewResult)
  ├── pyeza-golang/types      (TableConfig, TableColumn, TableRow)
  ├── pyeza-golang/route      (URL resolution helpers)
  └── esqyma                  (attachment protobuf types)
```

hybra depends **only** on framework packages (downward). It never imports domain packages.

## Adding a new cross-cutting feature

1. Create a new directory under `views/` (e.g., `views/comments/`)
2. Define a `Config` struct with dependency injection fields
3. Implement handler functions that return `view.View`
4. Add `Labels` struct + `DefaultLabels()` for UI text
5. Keep templates in `pyeza/templates/blocks/` or `pyeza/components/`
6. Domain packages create thin wrappers with entity-specific config

### Design principles

- **No domain knowledge** — hybra handlers are parameterized, never hard-coded to a specific entity
- **Dependency injection** — all storage, CRUD, and ID generation functions are injected via Config
- **Templates stay in pyeza** — hybra contains Go orchestration only, not HTML
- **Proto types are OK** — hybra can import esqyma proto types (it's a shared data contract, not domain logic)
- **Thin domain wrappers** — each domain's attachment.go is ~30 lines: just a Config + two exported functions

## Module

```
module github.com/erniealice/hybra-golang
```

## Etymology

**hybra** — from "hybrid," reflecting the package's role as a bridge between the pure UI framework and domain-specific business logic. It's the shared ground where cross-cutting application patterns live.
