// placement_test.go — Wave-T placement gate for hybra (CROSS-CUTTING variant, v2/Option B).
//
// Adopted verbatim from
// docs/orchestrate/20260610-package-cleanup/placement_test.template.go.
// ONLY the per-package config block below differs between packages; the shared
// logic must stay byte-identical so the gate is a single parameterized check
// (manifest §6 TT). Derives the esqyma domain set + per-domain ENTITY set LIVE
// from packages/esqyma/proto/v1/domain/ at test time.
//
// hybra is the §1a CROSS-CUTTING variant: it maps to several esqyma domains by
// concern (audit_trail, document, integration, communication/conversation), so
// the "mirror ONE esqyma domain" rule does NOT apply. The communication/
// conversation surface relocated here from entydad 2026-06-12 (view-package-
// placement.md OCID, manifest thread TC): a polymorphic, multi-view secure-
// messaging surface — a document.Attachment sibling, not OCID identity.
// crossCutting=true skips R1/R2/R2′/R3′/R5 and
// instead asserts every views/<x> is a chartered concern group and no
// framework-leak file sits at root; R4 (no god-files) still applies.
//
// hybra has NO block/usecases.go and NO RequireFor port struct — MustValidate
// is skipped-no-block.

package hybra_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ── per-package config (the ONLY part that differs between packages) ──────────
var crossCutting = true // hybra is the §1a cross-cutting carve-out.
// legacyAllow — EMPTY = STRICT (hybra has zero migration debt).
var legacyAllow = map[string]string{}
var charterViews = []string{"attachment", "auditlog", "conversation", "integration", "template"} // chartered concern groups
// subContexts: NAVIGATION-ONLY folders chartered directly under a single domain
// (domain/<d>/<subcontext>/<entity>/). EMPTY for hybra — cross-cutting variant has
// no domain/ subtree. Only entydad charters sub-contexts; this stays declared so
// the SHARED R2′ logic is byte-identical across all six packages.
var subContexts = []string{}

// ── shared logic — DO NOT EDIT per package ───────────────────────────────────

const godFileThreshold = 1200

// allowedFirstLevelDirs are the non-domain first-level dirs a domain package may
// hold besides domain/. Note BOTH "service" and "services" are infra surfaces
// (cyta uses the plural for its private recurrence/availability helpers).
var allowedFirstLevelDirs = map[string]bool{
	"domain":   true,
	"block":    true,
	"assets":   true,
	"service":  true,
	"services": true,
	"scripts":  true,
	"internal": true,
	"tests":    true,
	"web":      true,
}

// frameworkLeakFiles must never appear at a cross-cutting package's root — these
// concerns belong in pyeza (manifest §3 Wave P).
var frameworkLeakFiles = map[string]bool{
	"htmx.go":        true,
	"assets.go":      true,
	"datasource.go":  true,
	"package_dir.go": true,
	"pkgdir.go":      true,
}

// locateEsqymaDomain walks up from the CWD looking for the esqyma proto domain
// dir, then derives the domain set + the per-domain ENTITY set from its layout.
//
// esqyma defines a domain's entities one of two ways:
//   - as subdirs (the common case: operation/job/, operation/job_activity/, …)
//   - as top-level *.proto files when the domain has NO subdirs (fulfillment,
//     ping) — there the entity is the proto basename, EXCLUDING service defs
//     (*_service.proto) and the shared enums.proto.
//
// Returns domainEntities: domain -> set of that domain's entity names. This is a
// PER-DOMAIN set (not a flat entity→domain map), because entity names collide
// across domains (collection ∈ {product, treasury}; dashboard/enums/reporting
// recur) and Option B's R2′ asks "is <e> an entity OF THIS domain <d>", which a
// flat map cannot answer.
func locateEsqymaDomain(t *testing.T) (root string, domainSet map[string]bool, domainEntities map[string]map[string]bool) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("placement: cannot get cwd: %v", err)
	}
	rel := filepath.Join("proto", "v1", "domain")
	var candidate string
	// Prefer a sibling esqyma checkout (packages/<pkg>/.. == packages/), then
	// walk up looking for packages/esqyma/proto/v1/domain or esqyma/proto/...
	dir := cwd
	for {
		for _, c := range []string{
			filepath.Join(dir, "..", "esqyma", rel),
			filepath.Join(dir, "packages", "esqyma", rel),
			filepath.Join(dir, "esqyma", rel),
		} {
			if fi, err := os.Stat(c); err == nil && fi.IsDir() {
				candidate = c
				break
			}
		}
		if candidate != "" {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if candidate == "" {
		t.Fatalf("placement: could not locate packages/esqyma/proto/v1/domain from %s — the gate cannot run without the esqyma source of truth", cwd)
	}
	root = filepath.Clean(candidate)

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("placement: cannot read esqyma domain dir %s: %v", root, err)
	}
	domainSet = map[string]bool{}
	domainEntities = map[string]map[string]bool{}
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		domain := de.Name()
		domainSet[domain] = true
		domainEntities[domain] = map[string]bool{}
		ents, err := os.ReadDir(filepath.Join(root, domain))
		if err != nil {
			continue
		}
		sawSubdir := false
		for _, e := range ents {
			if e.IsDir() {
				sawSubdir = true
				domainEntities[domain][e.Name()] = true
			}
		}
		// Fallback: domains with no entity subdirs (fulfillment, ping) define
		// their entities as top-level *.proto files.
		if !sawSubdir {
			for _, e := range ents {
				name := e.Name()
				if e.IsDir() || !strings.HasSuffix(name, ".proto") {
					continue
				}
				stem := strings.TrimSuffix(name, ".proto")
				if strings.HasSuffix(stem, "_service") || stem == "enums" {
					continue // gRPC service def / shared enums — not an entity
				}
				domainEntities[domain][stem] = true
			}
		}
	}
	if len(domainSet) == 0 {
		t.Fatalf("placement: esqyma domain dir %s has no domains", root)
	}
	return root, domainSet, domainEntities
}

// moduleRoot returns the directory holding the package's go.mod (the placement
// gate's module root). It walks up from CWD; CWD is already the module root for a
// root-level placement_test.go, but walking up keeps the test correct if it is
// hosted in a subdir (e.g. internal/structure/).
func moduleRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("placement: cannot get cwd: %v", err)
	}
	dir := cwd
	for {
		if fi, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && !fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("placement: no go.mod found walking up from %s", cwd)
		}
		dir = parent
	}
}

// inLegacyAllow reports whether a path (relative to module root) is excused by
// legacyAllow — matched on its FULL slash-relative path, its FIRST path segment,
// OR its basename. The full-path form lets a precise multi-segment key
// (`domain/operation/outcome_summary`) excuse exactly one entity dir without the
// broad basename form accidentally excusing every same-named file. That is the
// shrinking migration ledger: each wave deletes entries; the capstone empties it.
func inLegacyAllow(relPath string) bool {
	if len(legacyAllow) == 0 {
		return false
	}
	relPath = filepath.ToSlash(relPath)
	if _, ok := legacyAllow[relPath]; ok {
		return true
	}
	first := relPath
	if i := strings.IndexByte(relPath, '/'); i >= 0 {
		first = relPath[:i]
	}
	if _, ok := legacyAllow[first]; ok {
		return true
	}
	base := filepath.Base(relPath)
	_, ok := legacyAllow[base]
	return ok
}

// labelsRoutesTypeDecls parses a .go file and returns the names of exported type
// decls whose name ends in Labels or Routes, split into two buckets:
//   - real:  `type XxxLabels struct{…}` / `type XxxRoutes …` (a fresh type)
//   - alias: `type XxxLabels = pkg.Labels` (an alias — the facade's re-export)
//
// R3′ forbids `real` contract types at a domain root (they must live in an
// entity dir) but allows `alias` (that IS the facade). An *ast.TypeSpec carries
// Assign != token.NoPos iff it is an alias (`type X = Y`), which is exactly how
// the Go spec distinguishes the two.
func labelsRoutesTypeDecls(path string) (real, alias []string, err error) {
	fset := token.NewFileSet()
	f, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if perr != nil {
		return nil, nil, perr
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				continue
			}
			name := ts.Name.Name
			if !strings.HasSuffix(name, "Labels") && !strings.HasSuffix(name, "Routes") {
				continue
			}
			if ts.Assign.IsValid() { // `type X = Y` — an alias (facade re-export)
				alias = append(alias, name)
			} else { // `type X struct{…}` — a real contract type decl
				real = append(real, name)
			}
		}
	}
	return real, alias, nil
}

func countLines(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, nil
	}
	n := strings.Count(string(data), "\n")
	if data[len(data)-1] != '\n' {
		n++ // last line without trailing newline
	}
	return n, nil
}

func isGoFile(name string) bool   { return strings.HasSuffix(name, ".go") }
func isTestFile(name string) bool { return strings.HasSuffix(name, "_test.go") }

func TestPlacement(t *testing.T) {
	root := moduleRoot(t)
	_, domainSet, domainEntities := locateEsqymaDomain(t)

	// R4 (all variants): no god-files anywhere (excl. *_test.go), unless excused.
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isGoFile(info.Name()) || isTestFile(info.Name()) {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if inLegacyAllow(rel) {
			return nil
		}
		n, cerr := countLines(path)
		if cerr != nil {
			return nil
		}
		if n > godFileThreshold {
			t.Errorf("%s: %d lines exceeds the %d god-file threshold — split per entity", rel, n, godFileThreshold)
		}
		return nil
	})

	if crossCutting {
		runCrossCutting(t, root)
	} else {
		runDomainVariant(t, root, domainSet, domainEntities)
	}

	if testing.Verbose() {
		if len(legacyAllow) == 0 {
			t.Logf("placement: legacyAllow EMPTY — STRICT gate (no remaining migration debt)")
		} else {
			keys := make([]string, 0, len(legacyAllow))
			for k := range legacyAllow {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			t.Logf("placement: legacyAllow remaining debt (%d) — each MUST carry a dated EXPIRES stamp:", len(keys))
			for _, k := range keys {
				t.Logf("  - %s: %s", k, legacyAllow[k])
			}
		}
	}
}

func runDomainVariant(t *testing.T, root string, domainSet map[string]bool, domainEntities map[string]map[string]bool) {
	// R1 Empty root: no package .go directly at module root (only *_test.go).
	rootEntries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("placement: cannot read module root %s: %v", root, err)
	}
	for _, de := range rootEntries {
		if de.IsDir() || !isGoFile(de.Name()) || isTestFile(de.Name()) {
			continue
		}
		if inLegacyAllow(de.Name()) {
			continue
		}
		t.Errorf("%s: root holds no package code — re-home (→ domain/<d>/, → pyeza, or owning pkg)", de.Name())
	}

	// R2 Canonical domains: every first-level dir is an allowed infra dir or
	// `domain`; every subdir of domain/ is an esqyma proto domain.
	for _, de := range rootEntries {
		if !de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if inLegacyAllow(name) {
			continue
		}
		if name == "domain" || allowedFirstLevelDirs[name] {
			continue
		}
		t.Errorf("%s/: not an esqyma proto domain — fold into the owning domain or a service/ surface", name)
	}
	domainDir := filepath.Join(root, "domain")
	domEntries, derr := os.ReadDir(domainDir)
	if derr != nil {
		return // no domain/ dir — R1/R2 above already covered the surface
	}
	for _, de := range domEntries {
		if !de.IsDir() {
			continue
		}
		d := de.Name()
		rel := filepath.ToSlash(filepath.Join("domain", d))
		if inLegacyAllow(rel) || inLegacyAllow(d) {
			continue
		}
		if !domainSet[d] {
			t.Errorf("%s/: not an esqyma proto domain — fold into the owning domain or a service/ surface", rel)
		}
	}

	// R2′ + R3′ + R5: per esqyma domain present in domain/.
	for _, de := range domEntries {
		if !de.IsDir() {
			continue
		}
		d := de.Name()
		if !domainSet[d] {
			continue // R2 already flagged it
		}
		ddir := filepath.Join(domainDir, d)
		entities := domainEntities[d] // may be empty for an entity-less domain

		ddEntries, err := os.ReadDir(ddir)
		if err != nil {
			continue
		}

		// R5: a facade domain/<d>/<d>.go must exist for every domain dir that
		// holds ≥1 entity subdir. Completeness is build-enforced, so we only
		// check the FILE's presence.
		hasEntitySubdir := false
		for _, child := range ddEntries {
			if child.IsDir() && isEntityDir(child.Name(), d, entities) {
				hasEntitySubdir = true
				break
			}
		}
		facade := filepath.Join(ddir, d+".go")
		facadeRel := filepath.ToSlash(filepath.Join("domain", d, d+".go"))
		if hasEntitySubdir && !inLegacyAllow(facadeRel) && !inLegacyAllow(d+".go") {
			if _, ferr := os.Stat(facade); ferr != nil {
				t.Errorf("domain/%s/: missing facade %s.go (package %s) — R5: every domain with ≥1 entity needs a hand-written facade", d, d, d)
			}
		}

		// R2′: every DIR directly under domain/<d>/ is a valid entity dir, or
		// `shared`, or a disambiguated domain-level view (name starts with <d>),
		// OR — for a package that charters them (subContexts) — a navigation-only
		// sub-context folder whose CHILD dirs are each validated as entities of
		// <d> (recurse exactly ONE level). subContexts is empty for every package
		// except entydad, so the recursion branch is a no-op everywhere else.
		subCtx := map[string]bool{}
		for _, s := range subContexts {
			subCtx[s] = true
		}
		for _, child := range ddEntries {
			if !child.IsDir() {
				continue // sibling files (<d>.go, <e>_module.go, routes.go, labels.go) — not R2′'s concern
			}
			cn := child.Name()
			crel := filepath.ToSlash(filepath.Join("domain", d, cn))
			if inLegacyAllow(crel) || inLegacyAllow(cn) {
				continue
			}
			if subCtx[cn] {
				// Chartered NAVIGATION-ONLY sub-context (e.g. entydad
				// domain/entity/party/): the folder itself is not an entity —
				// recurse ONE level and validate each grandchild as an entity of
				// <d>. The sub-context layer is invisible to the entity contract.
				subEntries, serr := os.ReadDir(filepath.Join(ddir, cn))
				if serr != nil {
					continue
				}
				for _, gc := range subEntries {
					if !gc.IsDir() {
						continue // sub-context sibling files (<e>_module.go, helpers.go) — not R2′'s concern
					}
					gn := gc.Name()
					grel := filepath.ToSlash(filepath.Join("domain", d, cn, gn))
					if inLegacyAllow(grel) || inLegacyAllow(gn) {
						continue
					}
					if validEntityChild(gn, d, entities) {
						continue // an esqyma entity / shared / domain-view of THIS domain
					}
					t.Errorf("domain/%s/%s/%s/: not an esqyma entity of domain %q, not shared/, not a domain-view (%s*) — R2′: the entity is the package unit (under the %q sub-context)", d, cn, gn, d, d, cn)
				}
				continue
			}
			if validEntityChild(cn, d, entities) {
				continue // an esqyma entity / shared / domain-view of THIS domain
			}
			t.Errorf("domain/%s/%s/: not an esqyma entity of domain %q, not shared/, not a domain-view (%s*) — R2′: the entity is the package unit", d, cn, d, d)
		}

		// R3′: at the DOMAIN ROOT (files directly in domain/<d>/, NOT in an
		// entity subdir), no REAL exported *Labels/*Routes type declaration may
		// exist — contract types live in their entity dir. Alias re-exports
		// (`type JobLabels = job.Labels`) ARE the facade and are allowed.
		for _, child := range ddEntries {
			if child.IsDir() || !isGoFile(child.Name()) || isTestFile(child.Name()) {
				continue
			}
			frel := filepath.ToSlash(filepath.Join("domain", d, child.Name()))
			if inLegacyAllow(frel) || inLegacyAllow(child.Name()) {
				continue
			}
			real, _, perr := labelsRoutesTypeDecls(filepath.Join(ddir, child.Name()))
			if perr != nil {
				continue
			}
			for _, typeName := range real {
				t.Errorf("%s: %s is a contract TYPE declared at the domain root — R3′: move it into its own domain/%s/<entity>/ (the domain root holds only the facade's alias re-exports)", frel, typeName, d)
			}
		}
	}
}

// validEntityChild reports whether a dir named cn (a direct child of domain/<d>/,
// or — for a chartered sub-context — a grandchild domain/<d>/<subcontext>/cn) is
// an acceptable entity-level package: an esqyma entity of <d>, the cross-entity
// `shared` leaf, or a disambiguated domain-level view (name starts with <d>). It
// is the single predicate both the direct and the one-level-recursed R2′ paths
// call, so the sub-context recursion validates by the IDENTICAL rule.
func validEntityChild(cn, d string, entities map[string]bool) bool {
	return cn == "shared" || isEntityDir(cn, d, entities) || isDomainView(cn, d)
}

// isEntityDir reports whether a dir named cn directly under domain/<d>/ is a
// valid Option-B entity package: cn is an esqyma entity of domain d. The
// special case cn == d (the domain-named entity, e.g. fulfillment/fulfillment,
// event/event) is accepted because esqyma carries an entity of the domain's own
// name in those domains.
func isEntityDir(cn, d string, entities map[string]bool) bool {
	if entities[cn] {
		return true
	}
	// A domain-named entity dir (fulfillment/fulfillment) is valid iff esqyma
	// has an entity of that name in this domain — already covered by entities[cn]
	// when proto-derived. This branch is a no-op guard kept for clarity.
	return false
}

// isDomainView reports whether a dir is a disambiguated domain-level view
// package (§4.3): not an esqyma entity, but a domain-scoped dashboard/settings
// view whose package name is prefixed with the domain to avoid the bare
// `package dashboard` collision across 31 dirs. Convention: the dir name starts
// with the domain name and is longer than it (operationdashboard, ledgersettings).
func isDomainView(cn, d string) bool {
	return strings.HasPrefix(cn, d) && len(cn) > len(d)
}

func runCrossCutting(t *testing.T, root string) {
	// No framework-leak files at root.
	if rootEntries, err := os.ReadDir(root); err == nil {
		for _, de := range rootEntries {
			if de.IsDir() {
				continue
			}
			if frameworkLeakFiles[de.Name()] && !inLegacyAllow(de.Name()) {
				t.Errorf("%s: framework concern leaked to root — belongs in pyeza (Wave P)", de.Name())
			}
		}
	}
	// Every subdir of views/ must be a chartered concern group.
	charter := map[string]bool{}
	for _, c := range charterViews {
		charter[c] = true
	}
	viewsDir := filepath.Join(root, "views")
	if vEntries, err := os.ReadDir(viewsDir); err == nil {
		for _, de := range vEntries {
			if !de.IsDir() {
				continue
			}
			name := de.Name()
			if inLegacyAllow(filepath.ToSlash(filepath.Join("views", name))) || inLegacyAllow(name) {
				continue
			}
			if !charter[name] {
				t.Errorf("views/%s/: not a chartered cross-cutting concern group — expected one of %v", name, charterViews)
			}
		}
	}
}
