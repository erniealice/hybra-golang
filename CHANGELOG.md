# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0-alpha] - 2026-06-15

Cross-cutting shared view handlers — first published alpha.

### Added
- Shared view handlers: attachments, audit-trail operations, conversations, and
  document-template CRUD.

### Changed
- `go.mod` now references published tags (`pyeza-golang`, `esqyma`, `espyna-golang`
  at `v0.1.0-alpha`) instead of local `replace` directives; local development
  continues via `go.work`.

[Unreleased]: https://github.com/erniealice/hybra-golang/compare/v0.1.0-alpha...HEAD
[0.1.0-alpha]: https://github.com/erniealice/hybra-golang/releases/tag/v0.1.0-alpha
