// wirelint: enforce constraint C-006 — flag literal occurrences of any
// GraphQL wire-name (declared in defquery IR docs) outside excluded
// generated-client paths.
//
// Usage:
//   wirelint --ir-dir <contracts/ir/domain> --root <target-tree> [--exclude DIR,DIR2] [--json]
//
// Exit codes:
//   0 — OK (no violations, or no wire-names declared yet)
//   1 — at least one violation
//   2 — misuse / I/O error
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/wirelint"
)

func main() {
	defaultIR := ""
	if root, err := findroot.FromCWD(); err == nil {
		defaultIR = filepath.Join(root, "ir", "domain")
	}
	irDir := flag.String("ir-dir", defaultIR, "directory containing *.ir.json files")
	root := flag.String("root", ".", "tree to scan for wire-name literals")
	excludeCSV := flag.String("exclude", "", "comma-separated relative dirs allowed to contain literals (e.g. internal/contracts,lib/contracts_client)")
	asJSON := flag.Bool("json", false, "JSON output")
	flag.Parse()

	if *irDir == "" {
		fmt.Fprintln(os.Stderr, "--ir-dir is required (could not auto-detect contracts root)")
		os.Exit(2)
	}

	wires, err := wirelint.LoadWires(*irDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load wires:", err)
		os.Exit(2)
	}
	excludes := splitCSV(*excludeCSV)
	findings, err := wirelint.Scan(*root, wires, excludes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(2)
	}

	if *asJSON {
		out := map[string]any{
			"ok":       len(findings) == 0,
			"root":     *root,
			"excludes": excludes,
			"wires":    wires,
			"findings": findings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Println(wirelint.Summary(wires, findings))
		if len(findings) > 0 {
			fmt.Fprintln(os.Stderr, "\nViolations (constraint C-006):")
			for _, f := range findings {
				fmt.Fprintf(os.Stderr, "  %s:%d  wire=%q  %s\n", f.Path, f.Line, f.Wire, f.Excerpt)
			}
			fmt.Fprintln(os.Stderr, "\nFix: reference the generated client constant instead of the literal.")
			fmt.Fprintln(os.Stderr, "  Go:    contracts.<Name>QueryName  (in agent-cluster-backend)")
			fmt.Fprintln(os.Stderr, "  Dart:  <camelName>QueryName       (in agent-cluster-frontend)")
			fmt.Fprintln(os.Stderr, "If a literal is intentional (test, doc), append wirelint:ignore on the same line.")
		}
	}
	if len(findings) > 0 {
		os.Exit(1)
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
