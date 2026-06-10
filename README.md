# hybra-golang

**Cross-cutting shared view handlers for the Ichizen OS monorepo.**

hybra is the §1a cross-cutting carve-out: it spans multiple esqyma domains (`audit_trail`, `document`, `integration`) rather than owning a single one. All domain packages depend downward on hybra; hybra never imports a domain package.

**Module path:** `github.com/erniealice/hybra-golang`

## Why hybra is cross-cutting

hybra holds features that are polymorphic across entities and domains — things like file attachments (any entity type), system-wide audit trail rendering, document templates, and integration composition. No single domain should own these, and they must not live in the UI framework (`pyeza`). hybra is the neutral home.

```
centymo, entydad, fycha, cyta, fayna   (domain packages)
        │   all depend downward on hybra
        ▼
     hybra-golang                      (cross-cutting — this package)
        │
        ▼
     pyeza-golang, esqyma              (framework / schema)
```

## Package structure

```
hybra-golang/
  placement_test.go    # STRICT cross-cutting placement gate (see below)
  labels.go            # root contract — package hybra
  routes.go            # root contract — package hybra
  routes_config.go     # root contract — package hybra
  go.mod
  go.sum
  views/
    attachment/        # polymorphic file-attachment handler (14+ entity types)
      handler.go
      handler_test.go
      ops.go
      policy.go
      policy_test.go
      form/form.go
    auditlog/          # system-wide audit trail view ops
      ops.go
    integration/       # integration composition surface
      module.go
      embed.go
      dashboard/page.go
      templates/dashboard.html
    template/          # document template CRUD handlers
      handler.go
      form/form.go
```

The root `labels.go`, `routes.go`, and `routes_config.go` are **not god-files** — they are small contract files in `package hybra` that expose cross-package types. Their presence at root is intentional for the cross-cutting variant (the placement gate explicitly permits them).

## Placement gate (`placement_test.go`)

hybra carries a **STRICT** cross-cutting placement gate (`legacyAllow` is empty — zero migration debt). It runs as `go test ./...` and uses the cross-cutting variant (`crossCutting = true`):

| Rule | What it checks |
|------|---------------|
| **R4** No god-files | No `.go` file (excl. `_test.go`) may exceed 1,200 lines — applies to all variants |
| **Charter check** | Every `views/<x>/` must be one of the four chartered concern groups; no ad-hoc directories |
| **Framework-leak check** | Files like `htmx.go`, `assets.go`, `datasource.go`, `pkgdir.go` must not appear at root — those belong in pyeza |

The "one esqyma domain" rules (R1/R2/R3) are **skipped** for cross-cutting packages. The charter is:

```go
charterViews = []string{"attachment", "auditlog", "integration", "template"}
```

## Chartered concern groups

| Directory | Cross-cutting concern | Spans esqyma domain |
|-----------|-----------------------|---------------------|
| `views/attachment/` | Polymorphic file attachments for any entity | `document` |
| `views/auditlog/` | System-wide audit trail viewer | `audit_trail` |
| `views/integration/` | Integration composition surface | `integration` |
| `views/template/` | Document template CRUD | `document` |

## What does NOT live in hybra

| Concern | Where it belongs |
|---------|-----------------|
| HTML templates | `pyeza/templates/blocks/` — pure presentation |
| CSS / JS | `pyeza/styles/`, `pyeza/assets/js/` — client-side behavior |
| Domain-specific upload workflows | Domain packages (e.g., centymo invoice template upload) |
| Proto schemas | `esqyma/proto/v1/domain/document/attachment/` |
| Storage adapters (S3, GCS, local) | `espyna/` — infrastructure |
| Entity-specific Config wrappers | Domain packages — thin wiring only |

## Dependencies

- `github.com/erniealice/pyeza-golang` — view interface, types (TableConfig, etc.), route helpers
- `github.com/erniealice/esqyma` — proto types (attachment, audit_trail, document, integration)

hybra depends only on framework packages. It never imports domain packages.

## Role in the monorepo

hybra is the reference implementation of the §1a cross-cutting carve-out pattern. When a new feature is genuinely polymorphic across multiple domains (not owned by any single domain), it belongs here. Use the charter check in `placement_test.go` as the gate: a new `views/<concern>/` directory must first be added to `charterViews`.

See `docs/wiki/articles/package-map.md` for the full dependency graph.
