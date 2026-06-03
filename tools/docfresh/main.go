// docfresh check: PR-time guard that ensures decisions touching
// dumb-agent-visible rules also update AGENT_CONTRACT.md. Decision 025.
//
// Usage in CI (against PR base):
//
//	./bin/docfresh check --base origin/main
//
// Usage locally with a precomputed file list:
//
//	git diff --name-only origin/main...HEAD | ./bin/docfresh check --stdin
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/decision"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/docfresh"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "check":
		cmdCheck(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func cmdCheck(args []string) {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	base := fs.String("base", "origin/main", "base ref to diff against (ignored with --stdin)")
	head := fs.String("head", "HEAD", "head ref to diff against (ignored with --stdin)")
	useStdin := fs.Bool("stdin", false, "read changed file paths from stdin (one per line) instead of running git diff")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)

	root, err := findroot.FromCWD()
	if err != nil {
		die(err)
	}

	changed, err := changedFiles(*base, *head, *useStdin)
	if err != nil {
		die(err)
	}

	added, err := loadAddedDecisions(root, changed)
	if err != nil {
		die(err)
	}

	violations := docfresh.Check(added, changed)
	if *asJSON {
		msgs := make([]string, len(violations))
		for i, v := range violations {
			msgs[i] = v.Error()
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"ok":            len(violations) == 0,
			"changed_files": len(changed),
			"added_decisions": len(added),
			"violations":    msgs,
		})
	} else {
		if len(violations) == 0 {
			fmt.Printf("docfresh check: OK (%d changed files, %d added decisions, none required an unmade AGENT_CONTRACT.md update)\n", len(changed), len(added))
		} else {
			fmt.Fprintf(os.Stderr, "docfresh check: %d violation(s)\n", len(violations))
			for _, v := range violations {
				fmt.Fprintln(os.Stderr, "  -", v.Error())
			}
		}
	}
	if len(violations) > 0 {
		os.Exit(1)
	}
}

func changedFiles(base, head string, useStdin bool) ([]string, error) {
	if useStdin {
		var out []string
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			line := strings.TrimSpace(s.Text())
			if line == "" {
				continue
			}
			out = append(out, line)
		}
		if err := s.Err(); err != nil {
			return nil, err
		}
		return out, nil
	}
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=AM", base+"..."+head)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff %s...%s: %w", base, head, err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

func loadAddedDecisions(root string, changed []string) ([]*decision.Record, error) {
	var out []*decision.Record
	for _, f := range changed {
		if !docfresh.IsDecisionPath(f) {
			continue
		}
		// Load relative to repo root. The CLI is expected to run from
		// inside the contracts checkout (findroot resolves it).
		r, err := decision.Load(f)
		if err != nil {
			// A decision listed in the diff that doesn't load is itself
			// a problem the decision-validate step catches; skip here.
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `docfresh — enforce AGENT_CONTRACT.md freshness for dumb-agent-visible decisions (D025)

Commands:
  check [--base REF] [--head REF] [--stdin] [--json]
                    For each added decision in the PR diff, if any
                    guards[].ref references a dumb-agent-visible rule
                    (internal/probe, internal/agentguard, purpose-banlist,
                    agent-roles), require AGENT_CONTRACT.md to be in the diff.
                    Exit 1 on any violation.`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
