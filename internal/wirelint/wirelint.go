// Package wirelint enforces constraint C-006: GraphQL wire identifiers
// (declared in defquery DSL forms, projected through IR) must reach
// consumer code only via generated client constants, never as hand-typed
// string literals.
//
// Inputs:
//
//   - irDir: directory containing ir/domain/*.ir.json (only kind=query
//     entries contribute wire names).
//   - rootDir: tree to scan (typically the consumer repo).
//   - excludeDirs: paths inside rootDir where literals are allowed
//     (the generated client lives here). Patterns are matched against
//     forward-slash relative paths and treated as path prefixes.
//
// The check skips:
//
//   - Files whose first 5 lines contain "Code generated" (the marker
//     emitted by gen-go-client and gen-dart-client).
//   - Common binary extensions and tool dirs (.git, bin, vendor,
//     node_modules, .dart_tool, build, etc.).
//   - Files larger than 1 MiB.
//
// Finding a wire-name LITERAL inside a quoted string ("workItems" or
// 'workItems') outside excluded dirs is a violation.
package wirelint

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

// Wire is a wire-name SSOT entry loaded from IR.
type Wire struct {
	Name      string // e.g. "workItems"
	IRPath    string // ir/domain/list-work-items.ir.json (relative to irDir)
	IRDocName string // kebab name of the IR doc, e.g. "list-work-items"
}

// Finding is one violation: a literal occurrence of a wire-name outside
// allowed paths.
type Finding struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Wire    string `json:"wire"`
	Excerpt string `json:"excerpt"`
}

// LoadWires walks irDir and collects wire_name values from every IR doc
// whose kind is "query". Returns an empty slice (no error) when irDir has
// no query docs.
func LoadWires(irDir string) ([]Wire, error) {
	docs, err := codegen.LoadAll(irDir)
	if err != nil {
		return nil, err
	}
	var wires []Wire
	for _, d := range docs {
		if d.Kind != "query" {
			continue
		}
		if d.WireName == "" {
			continue
		}
		wires = append(wires, Wire{
			Name:      d.WireName,
			IRPath:    d.Path,
			IRDocName: d.Name,
		})
	}
	sort.Slice(wires, func(i, j int) bool { return wires[i].Name < wires[j].Name })
	return wires, nil
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

type compiledWire struct {
	Name string
	Re   *regexp.Regexp
}

// Scan walks rootDir and returns one Finding per quoted-literal occurrence
// of any wire-name outside excludeDirs. excludeDirs are relative path
// prefixes (e.g. "internal/contracts" or "lib/contracts_client").
func Scan(rootDir string, wires []Wire, excludeDirs []string) ([]Finding, error) {
	if len(wires) == 0 {
		return nil, nil
	}

	compiledWires := make([]compiledWire, 0, len(wires))
	for _, w := range wires {
		q := regexp.QuoteMeta(w.Name)
		re := regexp.MustCompile(`["']` + q + `["']`)
		compiledWires = append(compiledWires, compiledWire{Name: w.Name, Re: re})
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
			// Skip whole excluded directories.
			for _, e := range excl {
				if relSlash == e {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if skipExt[strings.ToLower(filepath.Ext(d.Name()))] {
			return nil
		}
		// Skip files inside excluded dirs (in case WalkDir entered).
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
		ff, err := scanFile(path, relSlash, compiledWires)
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

func scanFile(path, relSlash string, compiledWires []compiledWire) ([]Finding, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	lineNo := 0
	headerBuf := make([]string, 0, 5)
	var findings []Finding
	headerChecked := false
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if !headerChecked {
			headerBuf = append(headerBuf, line)
			if lineNo >= 5 || sc.Err() != nil {
				headerChecked = true
				if containsGenerated(headerBuf) {
					return nil, nil
				}
			}
		}
		// "wirelint:ignore" line marker for intentional literals (e.g. tests).
		if strings.Contains(line, "wirelint:ignore") {
			continue
		}
		for _, cw := range compiledWires {
			if cw.Re.FindStringIndex(line) != nil {
				findings = append(findings, Finding{
					Path:    relSlash,
					Line:    lineNo,
					Wire:    cw.Name,
					Excerpt: snippet(line),
				})
			}
		}
	}
	// File with fewer than 5 lines: still need to check generated marker.
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

// Summary is a short human-readable result line for the CLI.
func Summary(wires []Wire, findings []Finding) string {
	if len(wires) == 0 {
		return "wirelint: OK (no wire-names declared in IR yet)"
	}
	if len(findings) == 0 {
		return fmt.Sprintf("wirelint: OK (checked %d wire-name(s), no violations)", len(wires))
	}
	return fmt.Sprintf("wirelint: %d violation(s) of constraint C-006", len(findings))
}
