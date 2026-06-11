# hybra-golang

**Cross-cutting shared view handlers for the Ichizen OS monorepo.**

hybra is the §1a cross-cutting carve-out: it spans multiple esqyma proto domains
(`audit_trail`, `document`, `integration`) rather than owning a single one. All
domain packages depend downward on hybra; hybra never imports a domain package.

**Module path:** `github.com/erniealice/hybra-golang`

## Dependency position

```
centymo, entydad, fycha, cyta, fayna   (domain packages)
        │   all depend downward on hybra
        ▼
     hybra-golang                      (cross-cutting — this package)
        │
        ▼
     pyeza-golang, esqyma              (framework / schema)
```

## Package tree

```
hybra-golang/
  placement_test.go    # STRICT cross-cutting gate (crossCutting=true, legacyAllow={})
  labels.go            # root contract — IntegrationLabels, DefaultIntegrationLabels()
  routes.go            # root contract — IntegrationDashboardURL const
  routes_config.go     # root contract — IntegrationRoutes, DefaultIntegrationRoutes()
  go.mod / go.sum

  views/
    attachment/                    # polymorphic file-attachment handler (14+ entity types)
      handler.go                   #   HTTP handlers (upload / list / delete)
      ops.go                       #   use-case ops wiring
      policy.go                    #   policy helpers
      form/
        form.go                    #   template-facing form types
    auditlog/                      # system-wide audit trail viewer
      ops.go                       #   use-case ops wiring
    integration/                   # integration composition surface
      module.go                    #   Module + ModuleDeps — wires all integration views
      embed.go                     #   embed.FS for templates
      dashboard/
        page.go                    #   dashboard view handler + page data types
      templates/
        dashboard.html             #   integration dashboard template
    template/                      # document template CRUD
      handler.go                   #   HTTP handlers
      form/
        form.go                    #   template-facing form types
```

### Why views/ not domain/

Domain packages organize under `domain/<d>/<e>/` (entity as the contract unit)
because each domain maps 1:1 to an esqyma proto domain. hybra is polymorphic
across multiple domains, so the structure uses `views/<concern>/` concern groups
instead. The concern group is the contract unit — each group owns exactly one
cross-cutting feature.

The placement gate (`placement_test.go`, `crossCutting=true`) enforces this:
every `views/<x>/` must appear in `charterViews`; R1/R2/R2′/R3′/R5 (domain-
keyed rules) are skipped; R4 (no god-files ≥ 1200 lines) still applies.

### Module shape inside views/<concern>/

Each concern group follows the same Module shape used by domain packages:

- **`module.go`** — `ModuleDeps` + `Module` + `NewModule(deps)` + `RegisterRoutes(r)`.
  The consumer (service-admin block assembler) constructs deps, calls `NewModule`,
  and calls `RegisterRoutes` on the app's mux. This is the `<e>_module.go` hoisted
  pattern applied at concern-group level.
- **`page.go` / `handler.go`** — per-view `View` implementations (one file per
  URL endpoint or drawer).
- **`form/form.go`** — template-facing `Data`/`FormData` structs (no repo imports,
  no `Deps`).

### Root contract files

`labels.go`, `routes.go`, and `routes_config.go` live at the module root in
`package hybra`. They are small contract files — not god-files. Their root
placement is intentional for the cross-cutting variant: service-admin imports
`hybra.IntegrationLabels` and `hybra.IntegrationRoutes` directly (no sub-package
import for cross-cutting root types).

## Chartered concern groups

| Directory | Cross-cutting concern | Spans esqyma domain |
|-----------|-----------------------|---------------------|
| `views/attachment/` | Polymorphic file attachments (any entity) | `document` |
| `views/auditlog/` | System-wide audit trail viewer | `audit_trail` |
| `views/integration/` | Integration composition surface | `integration` |
| `views/template/` | Document template CRUD | `document` |

A new `views/<concern>/` directory requires a charter update: add the name to
`charterViews` in `placement_test.go` before creating the directory.

## What does NOT live in hybra

| Concern | Where it belongs |
|---------|-----------------|
| HTML templates | `pyeza/templates/blocks/` — pure presentation |
| CSS / JS | `pyeza/styles/`, `pyeza/assets/js/` — client-side behavior |
| Domain-specific upload workflows | Domain packages (e.g., centymo invoice upload) |
| Proto schemas | `esqyma/proto/v1/domain/document/attachment/` |
| Storage adapters (S3, GCS, local) | `espyna/` — infrastructure |
| Entity-specific config wrappers | Domain packages — thin wiring only |

## MustValidate / RequireFor

hybra has no `block/` directory and no `block/usecases.go` port struct —
`MustValidate` is **skipped-no-block**. hybra's `views/integration/module.go`
uses a nil-safe callback pattern (optional ports degrade to empty state) rather
than the fail-closed `RequireFor` wiring used by domain blocks.

## Dependencies

- `github.com/erniealice/pyeza-golang` — view interface, types, route helpers
- `github.com/erniealice/esqyma` — proto types (attachment, audit_trail, document, integration)

hybra depends only on framework packages. It never imports domain packages.

## Placement gate

hybra carries a **STRICT** cross-cutting placement gate (`legacyAllow` is empty):

| Rule | What it checks |
|------|----------------|
| **R4** No god-files | No `.go` file (excl. `_test.go`) may exceed 1,200 lines |
| **Charter check** | Every `views/<x>/` must be in `charterViews` |
| **Framework-leak check** | `htmx.go`, `assets.go`, `datasource.go`, `pkgdir.go` must not appear at root |

R1/R2/R2′/R3′/R5 (domain-keyed rules) are **skipped** for cross-cutting packages.

Run: `go test -run Placement ./...` from the module root.
