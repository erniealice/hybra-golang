// placement_test.go — Wave-T placement gate for hybra (CROSS-CUTTING variant).
//
// Adopted verbatim from
// docs/orchestrate/20260610-package-cleanup/placement_test.template.go.
// ONLY the per-package config block below differs between packages; the shared
// logic must stay byte-identical so the gate is a single parameterized check
// (manifest §6 TT). Derives the esqyma domain set + entity→domain map LIVE from
// packages/esqyma/proto/v1/domain/ at test time.
//
// hybra is the §1a CROSS-CUTTING variant: it maps to FOUR esqyma domains by
// concern (audit_trail, document, integration), so the "mirror ONE esqyma
// domain" rule does NOT apply. crossCutting=true skips R1/R2/R3 and instead
// asserts every views/<x> is a chartered concern group and no framework-leak
// file sits at root; R4 (no god-files) still applies. The small root contract
// files (labels.go/routes.go/routes_config.go, package hybra) are allowed — this
// file joins package hybra so it compiles alongside them. esqyma is found via
// the sibling ../esqyma/proto/v1/domain.

package hybra

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
var crossCutting = true                                                          // §1a cross-cutting carve-out
var legacyAllow = map[string]string{}                                            // EMPTY = STRICT (hybra is already compliant)
var charterViews = []string{"attachment", "auditlog", "integration", "template"} // chartered concern groups

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
// dir, then derives the domain set + entity→domain map from its subdirs.
func locateEsqymaDomain(t *testing.T) (root string, domainSet map[string]bool, entityDomain map[string]string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("placement: cannot get cwd: %v", err)
	}
	rel := filepath.Join("proto", "v1", "domain")
	var candidate string
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
	entityDomain = map[string]string{}
	for _, de := range entries {
		if !de.IsDir() {
			continue
		}
		domain := de.Name()
		domainSet[domain] = true
		ents, err := os.ReadDir(filepath.Join(root, domain))
		if err != nil {
			continue
		}
		for _, e := range ents {
			if e.IsDir() {
				entityDomain[e.Name()] = domain
			}
		}
	}
	if len(domainSet) == 0 {
		t.Fatalf("placement: esqyma domain dir %s has no domains", root)
	}
	return root, domainSet, entityDomain
}

// moduleRoot returns the directory holding the package's go.mod (the placement
// gate's module root). It walks up from CWD.
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
// legacyAllow — matched on its FIRST path segment OR its basename.
func inLegacyAllow(relPath string) bool {
	if len(legacyAllow) == 0 {
		return false
	}
	relPath = filepath.ToSlash(relPath)
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

// camelToSnake converts a CamelCase identifier prefix to snake_case.
func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r - 'A' + 'a')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// resolveEntity maps a CamelCase type prefix (before Labels/Routes) to the
// LONGEST known esqyma entity that is a segment-aligned snake_case prefix of it.
// EventTagButton -> event_tag (not event); returns "" if no known entity matches.
func resolveEntity(prefix string, entityDomain map[string]string) string {
	snake := camelToSnake(prefix)
	segs := strings.Split(snake, "_")
	best := ""
	for n := len(segs); n >= 1; n-- {
		cand := strings.Join(segs[:n], "_")
		if _, ok := entityDomain[cand]; ok {
			best = cand
			break // first hit is the longest by construction
		}
	}
	return best
}

// labelsRoutesTypes parses a .go file and returns exported type decls whose name
// ends in Labels or Routes.
func labelsRoutesTypes(path string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var out []string
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
			if strings.HasSuffix(name, "Labels") || strings.HasSuffix(name, "Routes") {
				out = append(out, name)
			}
		}
	}
	return out, nil
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
		n++
	}
	return n, nil
}

func isGoFile(name string) bool   { return strings.HasSuffix(name, ".go") }
func isTestFile(name string) bool { return strings.HasSuffix(name, "_test.go") }

func TestPlacement(t *testing.T) {
	root := moduleRoot(t)
	_, domainSet, entityDomain := locateEsqymaDomain(t)

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
		runDomainVariant(t, root, domainSet, entityDomain)
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
			t.Logf("placement: legacyAllow remaining debt (%d):", len(keys))
			for _, k := range keys {
				t.Logf("  - %s: %s", k, legacyAllow[k])
			}
		}
	}
}

func runDomainVariant(t *testing.T, root string, domainSet map[string]bool, entityDomain map[string]string) {
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

	// R2 Canonical domains.
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
	if domEntries, err := os.ReadDir(domainDir); err == nil {
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
	}

	// R3 Entity placement.
	if domEntries, err := os.ReadDir(domainDir); err == nil {
		for _, de := range domEntries {
			if !de.IsDir() {
				continue
			}
			d := de.Name()
			if !domainSet[d] {
				continue
			}
			ddir := filepath.Join(domainDir, d)
			_ = filepath.Walk(ddir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() || !isGoFile(info.Name()) || isTestFile(info.Name()) {
					return nil
				}
				rel, _ := filepath.Rel(root, path)
				if inLegacyAllow(rel) || inLegacyAllow(info.Name()) {
					return nil
				}
				types, perr := labelsRoutesTypes(path)
				if perr != nil {
					return nil
				}
				for _, typeName := range types {
					prefix := strings.TrimSuffix(strings.TrimSuffix(typeName, "Labels"), "Routes")
					if prefix == "" {
						continue
					}
					entity := resolveEntity(prefix, entityDomain)
					if entity == "" {
						if testing.Verbose() {
							t.Logf("placement: %s:%s — prefix %q maps to no esqyma entity (Surface/projection?), skipped by R3", rel, typeName, camelToSnake(prefix))
						}
						continue
					}
					owner := entityDomain[entity]
					if owner != "" && owner != d {
						t.Errorf("%s:%s: entity %s belongs to domain/%s/, found in domain/%s/", rel, typeName, entity, owner, d)
					}
				}
				return nil
			})
		}
	}
}

func runCrossCutting(t *testing.T, root string) {
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
