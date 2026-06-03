// probe — dumb-agent entry point for baseline and fixture verification.
//
// Subcommands:
//   probe preflight [--json]
//       Run all contracts-local baseline checks (decision validate,
//       ssotdeps verify --mode local, conceptmap verify, secretscan, irdrift).
//       Output:
//         status = candidate_allowed       → dumb-agent may create ONE small candidate
//         status = baseline_blocked        → dumb-agent must stop;
//                                             may file ONE reports/environment-blockers/<id>.md
//
//   probe fixtures [--json]
//       Walk fixtures/positive/** and fixtures/negative/** with their .meta.json
//       sidecars and verify each. v1 supports fixture_type=decision only.
//
// Exit codes:
//   0  OK   — dumb-agent may proceed to the next step
//   1  Not OK — dumb-agent must stop or fix
//   2  Misuse / I/O error
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/probe"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	root, err := findroot.FromCWD()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "preflight":
		cmdPreflight(root, os.Args[2:])
	case "fixtures":
		cmdFixtures(root, os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand:", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func cmdPreflight(root string, args []string) {
	fs := flag.NewFlagSet("preflight", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	res := probe.Preflight(root)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		fmt.Printf("status: %s\n", res.Status)
		fmt.Printf("candidate_allowed: %v\n", res.CandidateAllowed)
		fmt.Printf("next_allowed_action: %s\n\n", res.NextAllowedAction)
		fmt.Println("checks:")
		for _, c := range res.AllChecks {
			mark := "✓"
			if !c.OK {
				mark = "✗"
			}
			fmt.Printf("  %s  %-20s  %s\n", mark, c.Name, c.Summary)
		}
		if len(res.Blockers) > 0 {
			fmt.Fprintln(os.Stderr, "\nblockers:")
			for _, c := range res.Blockers {
				fmt.Fprintf(os.Stderr, "  - %s: %s\n", c.Name, c.Summary)
				if c.Detail != "" {
					for _, line := range splitLines(c.Detail) {
						fmt.Fprintf(os.Stderr, "      %s\n", line)
					}
				}
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Dumb-agent guidance:")
			fmt.Fprintln(os.Stderr, "  Baseline is RED. Do NOT create fixtures, fuzz corpus,")
			fmt.Fprintln(os.Stderr, "  IR candidates, or domain guard-candidate reports.")
			fmt.Fprintln(os.Stderr, "  You MAY create at most one reports/environment-blockers/<id>.md")
			fmt.Fprintln(os.Stderr, "  describing what's broken; then stop.")
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}

func cmdFixtures(root string, args []string) {
	fs := flag.NewFlagSet("fixtures", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	res, err := probe.VerifyFixtures(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		if len(res.Checks) == 0 {
			fmt.Println("probe fixtures: OK (no fixtures present)")
		} else {
			passed := 0
			for _, c := range res.Checks {
				mark := "✓"
				if !c.OK {
					mark = "✗"
				} else {
					passed++
				}
				fmt.Printf("%s  [%s] %s  %s\n", mark, c.Category, c.Path, c.Reason)
			}
			if res.OK {
				fmt.Printf("\nprobe fixtures: OK (%d/%d)\n", passed, len(res.Checks))
			} else {
				fmt.Fprintf(os.Stderr, "\nprobe fixtures: FAIL (%d/%d)\n", passed, len(res.Checks))
			}
		}
	}
	if !res.OK {
		os.Exit(1)
	}
}

func splitLines(s string) []string {
	out := []string{}
	for _, line := range []byte(s) {
		_ = line
	}
	// Simple split — keep dependencies tiny.
	current := ""
	for _, r := range s {
		if r == '\n' {
			if current != "" {
				out = append(out, current)
			}
			current = ""
			continue
		}
		current += string(r)
	}
	if current != "" {
		out = append(out, current)
	}
	return out
}

func usage() {
	fmt.Fprintln(os.Stderr, `probe — dumb-agent baseline + fixture verifier

Commands:
  preflight  [--json]   Run baseline checks (decision validate, ssotdeps
                        --mode local, conceptmap verify, secretscan,
                        irdrift). Reports candidate_allowed or
                        baseline_blocked.
  fixtures   [--json]   Verify fixtures/{positive,negative} against their
                        .meta.json sidecars. v1 supports decision only.`)
}
