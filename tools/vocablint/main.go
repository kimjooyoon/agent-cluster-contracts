// vocablint: enforce constraint C-001 — flag any redeclaration of a
// contract vocabulary term (aggregate or event name from IR) in
// non-generated code outside the configured generated-client paths.
//
// Usage:
//   vocablint --ir-dir <contracts/ir/domain> --root <target> [--exclude DIR,DIR2] [--json]
//
// Auto-detect (decision 013): if --ir-dir is omitted, the tool finds the
// contracts root via findroot.FromCWDOrEnv (walks up from cwd, then
// AGENT_CLUSTER_CONTRACTS env var).
//
// Exit codes:
//   0 OK (no violations, or no vocabulary terms declared yet)
//   1 at least one violation
//   2 misuse / I/O error
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/vocablint"
)

func main() {
	defaultIR := ""
	if root, err := findroot.FromCWDOrEnv(); err == nil {
		defaultIR = filepath.Join(root, "ir", "domain")
	}
	irDir := flag.String("ir-dir", defaultIR, "directory containing *.ir.json files")
	root := flag.String("root", ".", "tree to scan for vocabulary redeclarations")
	excludeCSV := flag.String("exclude", "", "comma-separated relative dirs allowed to declare vocab (e.g. internal/contracts,lib/contracts_client)")
	asJSON := flag.Bool("json", false, "JSON output")
	flag.Parse()

	if *irDir == "" {
		fmt.Fprintln(os.Stderr, "--ir-dir is required (could not auto-detect contracts root; set AGENT_CLUSTER_CONTRACTS)")
		os.Exit(2)
	}

	terms, err := vocablint.LoadTerms(*irDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "load terms:", err)
		os.Exit(2)
	}
	excludes := splitCSV(*excludeCSV)
	findings, err := vocablint.Scan(*root, terms, excludes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "scan:", err)
		os.Exit(2)
	}

	if *asJSON {
		out := map[string]any{
			"ok":       len(findings) == 0,
			"root":     *root,
			"excludes": excludes,
			"terms":    terms,
			"findings": findings,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
	} else {
		fmt.Println(vocablint.Summary(terms, findings))
		if len(findings) > 0 {
			fmt.Fprintln(os.Stderr, "\nViolations (constraint C-001):")
			for _, f := range findings {
				fmt.Fprintf(os.Stderr, "  %s:%d  term=%s  lang=%s  %s\n", f.Path, f.Line, f.Term, f.Lang, f.Excerpt)
			}
			fmt.Fprintln(os.Stderr, "\nFix: import the generated client type instead of declaring the term locally.")
			fmt.Fprintln(os.Stderr, "  Go:   import \".../internal/contracts\"; use contracts.<Name>")
			fmt.Fprintln(os.Stderr, "  Dart: import 'package:.../contracts_client/<file>.dart'; use the generated class")
			fmt.Fprintln(os.Stderr, "If the declaration is intentional (test alias), append vocablint:ignore on the same line.")
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
