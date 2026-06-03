// agentguard: enforce per-role allowed/forbidden path rules on a PR diff.
//
// Usage:
//   agentguard verify --role ROLE [--files FILE1,FILE2,...] [--from REF] [--to REF]
//   agentguard list
//   agentguard show ROLE
//
// File list source resolution (in priority order):
//   1. --files CSV
//   2. lines from stdin (one path per line) if --stdin
//   3. `git diff --name-only FROM...TO` when --from is set
//   4. `git diff --name-only HEAD~1 HEAD` as a default for local use
//
// Exit codes:
//   0 OK
//   1 violation(s) found
//   2 misuse (missing role, bad args, agent-roles.riido.json unreadable)
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/agentguard"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
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
	case "verify":
		cmdVerify(root, os.Args[2:])
	case "list":
		cmdList(root)
	case "show":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: agentguard show ROLE")
			os.Exit(2)
		}
		cmdShow(root, os.Args[2])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintln(os.Stderr, "unknown subcommand:", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func cmdVerify(root string, args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	role := fs.String("role", "", "role id (required)")
	filesCSV := fs.String("files", "", "comma-separated changed paths")
	fromStdin := fs.Bool("stdin", false, "read changed paths from stdin (one per line)")
	from := fs.String("from", "", "git ref to diff from (e.g. main)")
	to := fs.String("to", "HEAD", "git ref to diff to")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	if *role == "" {
		fmt.Fprintln(os.Stderr, "--role required")
		os.Exit(2)
	}
	roles, err := agentguard.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	r := roles.Lookup(*role)
	if r == nil {
		fmt.Fprintln(os.Stderr, "unknown role:", *role)
		os.Exit(2)
	}
	files, err := resolveFiles(*filesCSV, *fromStdin, *from, *to)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	vs := agentguard.Check(r, files)
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(map[string]any{
			"role":       *role,
			"ok":         len(vs) == 0,
			"files":      files,
			"violations": vs,
		})
	} else {
		if len(vs) == 0 {
			fmt.Printf("agentguard: OK (role=%s, %d file(s) checked)\n", *role, len(files))
		} else {
			fmt.Fprintf(os.Stderr, "agentguard: %d violation(s) for role %s:\n", len(vs), *role)
			for _, v := range vs {
				if v.Path == "" {
					fmt.Fprintf(os.Stderr, "  - %s\n", v.Reason)
				} else {
					fmt.Fprintf(os.Stderr, "  - %s: %s\n", v.Path, v.Reason)
				}
			}
		}
	}
	if len(vs) > 0 {
		os.Exit(1)
	}
}

func cmdList(root string) {
	roles, err := agentguard.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	for _, r := range roles.Roles {
		fmt.Printf("%-15s %s\n", r.ID, r.Label)
	}
}

func cmdShow(root, id string) {
	roles, err := agentguard.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	r := roles.Lookup(id)
	if r == nil {
		fmt.Fprintln(os.Stderr, "unknown role:", id)
		os.Exit(2)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(r)
}

func resolveFiles(csv string, fromStdin bool, fromRef, toRef string) ([]string, error) {
	if csv != "" {
		var out []string
		for _, p := range strings.Split(csv, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		return out, nil
	}
	if fromStdin {
		var out []string
		sc := bufio.NewScanner(os.Stdin)
		for sc.Scan() {
			p := strings.TrimSpace(sc.Text())
			if p != "" {
				out = append(out, p)
			}
		}
		return out, sc.Err()
	}
	if fromRef == "" {
		fromRef = "HEAD~1"
	}
	cmd := exec.Command("git", "diff", "--name-only", fromRef+"..."+toRef)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, `agentguard — enforce per-role path allowlists on PR diffs

Commands:
  verify --role ROLE [--files A,B | --stdin | --from REF [--to REF]] [--json]
  list
  show ROLE`)
}
