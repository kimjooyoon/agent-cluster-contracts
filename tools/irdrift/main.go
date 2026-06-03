// irdrift: re-run the SBCL emitter into a temp dir and compare against the
// committed ir/ tree. Exit 1 if anything differs, is missing, or is extra.
// This is the merge-blocking guard for concept-map constraint C-002 (every IR
// file must match its DSL source).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/irdrift"
)

func main() {
	asJSON := flag.Bool("json", false, "JSON output")
	flag.Parse()
	root, err := findroot.FromCWD()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	res, err := irdrift.Check(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		if res != nil && res.SBCLOutput != "" {
			fmt.Fprintln(os.Stderr, "--- SBCL output ---")
			fmt.Fprintln(os.Stderr, res.SBCLOutput)
		}
		os.Exit(2)
	}
	drift := len(res.Differs) + len(res.MissingInTemp) + len(res.ExtraInTemp)
	if *asJSON {
		out := map[string]any{
			"ok":              drift == 0,
			"differs":         pathsOf(res.Differs),
			"missing_in_temp": res.MissingInTemp,
			"extra_in_temp":   res.ExtraInTemp,
		}
		json.NewEncoder(os.Stdout).Encode(out)
	} else {
		if drift == 0 {
			fmt.Println("irdrift: OK (IR matches DSL)")
		} else {
			for _, d := range res.Differs {
				fmt.Fprintf(os.Stderr, "DRIFT: %s differs from regenerated\n", d.Path)
			}
			for _, p := range res.MissingInTemp {
				fmt.Fprintf(os.Stderr, "MISSING: %s exists committed but emitter no longer produces it\n", p)
			}
			for _, p := range res.ExtraInTemp {
				fmt.Fprintf(os.Stderr, "EXTRA: emitter produces %s but it is not committed\n", p)
			}
		}
	}
	if drift > 0 {
		os.Exit(1)
	}
}

func pathsOf(ds []irdrift.Diff) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Path
	}
	return out
}
