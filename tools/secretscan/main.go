// secretscan: walk given roots (default = cwd) and flag known credential
// patterns. Exits 1 if anything is found. Append "secretscan:ignore" on a line
// to suppress matches for that line (used for tests and intentional examples).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/secretscan"
)

func main() {
	asJSON := flag.Bool("json", false, "JSON output")
	flag.Parse()
	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}
	patterns := secretscan.DefaultPatterns()
	skip := secretscan.DefaultSkip()
	var all []secretscan.Finding
	for _, root := range roots {
		found, err := secretscan.Scan(root, patterns, skip)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(2)
		}
		all = append(all, found...)
	}
	if *asJSON {
		out := map[string]any{
			"ok":       len(all) == 0,
			"findings": all,
		}
		json.NewEncoder(os.Stdout).Encode(out)
	} else {
		for _, f := range all {
			fmt.Printf("%s:%d: %s: %s\n", f.Path, f.Line, f.Pattern, f.Excerpt)
		}
		if len(all) == 0 {
			fmt.Println("secretscan: OK (no findings)")
		} else {
			fmt.Fprintf(os.Stderr, "secretscan: %d finding(s)\n", len(all))
		}
	}
	if len(all) > 0 {
		os.Exit(1)
	}
}
