// Package vocablint enforces constraint C-001: no repo other than
// agent-cluster-contracts may define a domain vocabulary term (aggregate,
// entity, value-object, event, policy, lifecycle state, view).
//
// Inputs:
//
//   - irDir: directory containing ir/domain/*.ir.json. Aggregate and event
//     names contribute. Other kinds (query) do not — wire names are wirelint's
//     domain.
//   - rootDir: tree to scan.
//   - excludeDirs: paths inside rootDir where vocabulary declarations are
//     allowed (the generated client lives here).
//
// Skip rules mirror wirelint:
//   - Files whose first 5 lines contain "Code generated" are skipped
//     (the marker emitted by gen-go-client and gen-dart-client).
//   - Common binary extensions and tool dirs (.git, bin, vendor,
//     node_modules, .dart_tool, build, etc.).
//   - Files larger than 1 MiB.
//
// Detection is per-language:
//   - Go (.go):   `type <Pascal> ...` (struct, interface, alias, etc.).
//   - Dart (.dart): `class <Pascal>` or `abstract class <Pascal>`.
//
// Append `vocablint:ignore` on a line to suppress matches there (used
// for tests and intentional aliases — e.g. a test struct named the same
// as a contract aggregate inside a non-generated test file).
package vocablint

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/codegen"
)

// Term is one vocabulary name loaded from IR.
type Term struct {
	Pascal    string // e.g. "WorkItem"
	Kebab     string // e.g. "work-item" (the IR document name)
	Kind      string // "aggregate" or "event"
	IRPath    string // ir/domain/work-item.ir.json
}

// Finding is one violation: a redeclaration of a vocabulary name in a
// non-generated file outside excluded dirs.
type Finding struct {
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Term     string `json:"term"`     // the PascalCase name that matched
	Lang     string `json:"lang"`     // "go" or "dart"
	Excerpt  string `json:"excerpt"`
}

// LoadTerms walks irDir and returns every PascalCase name from kind=aggregate
// or kind=event IR documents. Queries are wirelint's domain — skipped here.
func LoadTerms(irDir string) ([]Term, error) {
	docs, err := codegen.LoadAll(irDir)
	if err != nil {
		return nil, err
	}
	var terms []Term
	for _, d := range docs {
		if d.Kind != "aggregate" && d.Kind != "event" {
			continue
		}
		terms = append(terms, Term{
			Pascal: codegen.Pascal(d.Name),
			Kebab:  d.Name,
			Kind:   d.Kind,
			IRPath: d.Path,
		})
	}
	sort.Slice(terms, func(i, j int) bool { return terms[i].Pascal < terms[j].Pascal })
	return terms, nil
}

var skipDirs = map[string]bool{
	".git":         true,
	"bin":          true,
	"vendor":       true,
	"node_modules": true,
	".dart_tool":   true,
	"build":        true,
	".gradle":      true,
	"Pods":         true,
	".idea":        true,
	".vscode":      true,
}

var skipExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".pdf": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".zip": true, ".gz": true, ".tar": true, ".tgz": true,
	".so": true, ".dylib": true, ".dll": true, ".exe": true,
	".class": true, ".jar": true, ".o": true, ".a": true,
	".ico": true, ".icns": true,
}

type langPattern struct {
	lang string
	tmpl string // %s gets the PascalCase term
}

// langPatterns are per-extension detection patterns. %s is replaced with the
// Pascal term; \b boundaries prevent partial-name matches.
var langPatterns = map[string]langPattern{
	".go":   {lang: "go",   tmpl: `\btype\s+%s\b`},
	".dart": {lang: "dart", tmpl: `\b(?:abstract\s+)?class\s+%s\b`},
}

// compiledPattern is a single language-specific regex bound to one Term.
type compiledPattern struct {
	Term *Term
	Re   *regexp.Regexp
	Lang string
}

// Scan walks rootDir and returns one Finding per vocabulary redeclaration.
func Scan(rootDir string, terms []Term, excludeDirs []string) ([]Finding, error) {
	if len(terms) == 0 {
		return nil, nil
	}

	// Pre-compile per-language regexes once.
	compiledByExt := map[string][]compiledPattern{}
	for ext, p := range langPatterns {
		for i := range terms {
			t := &terms[i]
			re := regexp.MustCompile(fmt.Sprintf(p.tmpl, regexp.QuoteMeta(t.Pascal)))
			compiledByExt[ext] = append(compiledByExt[ext], compiledPattern{Term: t, Re: re, Lang: p.lang})
		}
	}

	excl := normalizeExcludes(excludeDirs)

	var findings []Finding
	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(rootDir, path)
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			for _, e := range excl {
				if relSlash == e {
					return filepath.SkipDir
				}
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if skipExt[ext] {
			return nil
		}
		patterns := compiledByExt[ext]
		if len(patterns) == 0 {
			return nil
		}
		for _, e := range excl {
			if e != "" && strings.HasPrefix(relSlash, e+"/") {
				return nil
			}
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > 1<<20 {
			return nil
		}
		ff, err := scanFile(path, relSlash, patterns)
		if err != nil {
			return err
		}
		findings = append(findings, ff...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func scanFile(path, relSlash string, patterns []compiledPattern) ([]Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	lineNo := 0
	headerBuf := make([]string, 0, 5)
	headerChecked := false
	var findings []Finding
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if !headerChecked {
			headerBuf = append(headerBuf, line)
			if lineNo >= 5 {
				headerChecked = true
				if containsGenerated(headerBuf) {
					return nil, nil
				}
			}
		}
		if strings.Contains(line, "vocablint:ignore") {
			continue
		}
		for _, p := range patterns {
			if p.Re.FindStringIndex(line) != nil {
				findings = append(findings, Finding{
					Path:    relSlash,
					Line:    lineNo,
					Term:    p.Term.Pascal,
					Lang:    p.Lang,
					Excerpt: snippet(line),
				})
			}
		}
	}
	if !headerChecked {
		if containsGenerated(headerBuf) {
			return nil, nil
		}
	}
	return findings, sc.Err()
}

func containsGenerated(headerLines []string) bool {
	for _, l := range headerLines {
		if strings.Contains(l, "Code generated") {
			return true
		}
	}
	return false
}

func snippet(line string) string {
	const max = 120
	s := strings.TrimSpace(line)
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func normalizeExcludes(in []string) []string {
	out := make([]string, 0, len(in))
	for _, e := range in {
		e = filepath.ToSlash(strings.Trim(strings.TrimSpace(e), "/"))
		if e != "" {
			out = append(out, e)
		}
	}
	return out
}

// Summary is a short human-readable line for the CLI.
func Summary(terms []Term, findings []Finding) string {
	if len(terms) == 0 {
		return "vocablint: OK (no vocabulary terms declared in IR yet)"
	}
	if len(findings) == 0 {
		return fmt.Sprintf("vocablint: OK (checked %d term(s), no violations)", len(terms))
	}
	return fmt.Sprintf("vocablint: %d violation(s) of constraint C-001", len(findings))
}
