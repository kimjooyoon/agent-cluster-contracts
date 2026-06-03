// agentguard: enforce per-role path allowed/forbidden rules on a PR diff.
//
// Usage:
//   agentguard verify --role ROLE [--files FILE1,FILE2,...] [--from REF] [--to REF] [--stdin] [--json]
//   agentguard list
//   agentguard show --role ROLE [--json]
//
// File-list sources (priority):
//   1. --files CSV
//   2. --stdin (one path per line)
//   3. git diff --name-only FROM...TO  (FROM defaults to HEAD~1 if --from unset)
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
	case "merge-check":
		cmdMergeCheck(root, os.Args[2:])
	case "validate":
		cmdValidate(root, os.Args[2:])
	case "list":
		cmdList(root)
	case "show":
		cmdShow(root, os.Args[2:])
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
	res := agentguard.Check(r, files)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		printVerifyText(r, res)
	}
	if !res.OK {
		os.Exit(1)
	}
}

func printVerifyText(role *agentguard.Role, res agentguard.CheckResult) {
	if res.OK {
		fmt.Printf("agentguard: OK (role=%s, %d file(s) checked)\n", res.RoleID, len(res.Files))
		return
	}

	// Group violations by kind for readability. Order: max_files, FORBIDDEN, not_allowed.
	var maxFiles, forbidden, notAllowed []agentguard.Violation
	for _, v := range res.Violations {
		switch v.Kind {
		case agentguard.KindMaxFiles:
			maxFiles = append(maxFiles, v)
		case agentguard.KindForbidden:
			forbidden = append(forbidden, v)
		case agentguard.KindNotAllowed:
			notAllowed = append(notAllowed, v)
		}
	}

	fmt.Fprintf(os.Stderr, "agentguard: %d violation(s) for role %s (see agent-roles.riido.json)\n\n", len(res.Violations), res.RoleID)

	if len(maxFiles) > 0 {
		fmt.Fprintln(os.Stderr, "PR-level:")
		for _, v := range maxFiles {
			fmt.Fprintf(os.Stderr, "  - %s\n", v.Reason)
		}
		fmt.Fprintln(os.Stderr)
	}

	if len(forbidden) > 0 {
		fmt.Fprintln(os.Stderr, "FORBIDDEN (must never be touched by this role):")
		for _, v := range forbidden {
			fmt.Fprintf(os.Stderr, "  - %s\n", v.Path)
			fmt.Fprintf(os.Stderr, "      forbidden_paths pattern %q\n", v.MatchedPattern)
		}
		fmt.Fprintln(os.Stderr)
	}

	if len(notAllowed) > 0 {
		fmt.Fprintln(os.Stderr, "not allowed (path falls outside this role's allowed_paths):")
		for _, v := range notAllowed {
			fmt.Fprintf(os.Stderr, "  - %s\n", v.Path)
			switch {
			case v.MissingPrefix != "":
				fmt.Fprintf(os.Stderr, "      closest allowed: %q — would match if prefixed with %q\n", v.ClosestAllowed, v.MissingPrefix)
			case v.ClosestAllowed != "":
				fmt.Fprintf(os.Stderr, "      closest allowed: %q\n", v.ClosestAllowed)
			default:
				fmt.Fprintf(os.Stderr, "      no allowed pattern shares any prefix with this path\n")
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	if len(res.Hints) > 0 {
		fmt.Fprintln(os.Stderr, "Hints:")
		for _, h := range res.Hints {
			fmt.Fprintf(os.Stderr, "  • %s\n", h)
		}
		fmt.Fprintln(os.Stderr)
	}

	fmt.Fprintln(os.Stderr, "Role summary:")
	fmt.Fprintf(os.Stderr, "  allowed_paths   = %v\n", role.AllowedPaths)
	fmt.Fprintf(os.Stderr, "  forbidden_paths = %v\n", role.ForbiddenPaths)
	if role.MaxFilesPerPR > 0 {
		fmt.Fprintf(os.Stderr, "  max_files_per_pr= %d\n", role.MaxFilesPerPR)
	}
}

func cmdMergeCheck(root string, args []string) {
	fs := flag.NewFlagSet("merge-check", flag.ExitOnError)
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
	res := agentguard.MergeCheck(r, files)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		if res.Allowed {
			fmt.Printf("agentguard merge-check: %s (role=%s, %d file(s))\n", res.Status, res.RoleID, len(res.Files))
		} else {
			fmt.Fprintf(os.Stderr, "agentguard merge-check: %s (role=%s)\n", res.Status, res.RoleID)
			for _, reason := range res.Reasons {
				fmt.Fprintln(os.Stderr, "  -", reason)
			}
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, "Role merge config:")
			fmt.Fprintf(os.Stderr, "  auto_merge_paths = %v\n", r.AutoMergePaths)
			fmt.Fprintf(os.Stderr, "  forbidden_paths  = %v\n", r.ForbiddenPaths)
			if r.MaxFilesPerPR > 0 {
				fmt.Fprintf(os.Stderr, "  max_files_per_pr = %d\n", r.MaxFilesPerPR)
			}
		}
	}
	if !res.Allowed {
		os.Exit(1)
	}
}

func cmdValidate(root string, args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	roles, err := agentguard.Load(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
	errs := agentguard.ValidateRoles(roles)
	if *asJSON {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"ok": len(errs) == 0, "errors": msgs})
	} else {
		if len(errs) == 0 {
			fmt.Printf("agentguard validate: OK (%d role(s))\n", len(roles.Roles))
		} else {
			fmt.Fprintf(os.Stderr, "agentguard validate: %d error(s)\n", len(errs))
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "  -", e)
			}
		}
	}
	if len(errs) > 0 {
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

func cmdShow(root string, args []string) {
	fs := flag.NewFlagSet("show", flag.ExitOnError)
	role := fs.String("role", "", "role id (required)")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	// Backward-compat: positional ROLE if --role unset.
	if *role == "" && fs.NArg() == 1 {
		*role = fs.Arg(0)
	}
	if *role == "" {
		fmt.Fprintln(os.Stderr, "usage: agentguard show --role ROLE [--json]")
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
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(r)
		return
	}
	fmt.Printf("Role: %s\n", r.ID)
	fmt.Printf("Label: %s\n", r.Label)
	if r.Decision != "" {
		fmt.Printf("Decision: %s\n", r.Decision)
	}
	if r.MaxFilesPerPR > 0 {
		fmt.Printf("Max files per PR: %d\n", r.MaxFilesPerPR)
	}
	fmt.Printf("PR isolation: candidate_only=%v guard_only=%v\n", r.PRIsolation.CandidateOnly, r.PRIsolation.GuardOnly)
	if r.PRIsolation.Rationale != "" {
		fmt.Printf("  rationale: %s\n", r.PRIsolation.Rationale)
	}
	fmt.Println()
	fmt.Println("Allowed paths:")
	for _, p := range r.AllowedPaths {
		fmt.Printf("  - %s\n", p)
	}
	fmt.Println()
	fmt.Println("Forbidden paths:")
	for _, p := range r.ForbiddenPaths {
		fmt.Printf("  - %s\n", p)
	}
	if r.AutoMergeEligible || len(r.AutoMergePaths) > 0 {
		fmt.Println()
		fmt.Printf("Auto-merge: eligible=%v\n", r.AutoMergeEligible)
		for _, p := range r.AutoMergePaths {
			fmt.Printf("  - %s\n", p)
		}
	}
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
  verify       --role ROLE [--files A,B | --stdin | --from REF [--to REF]] [--json]
                  Per-PR diff allowlist check (write authority).
  merge-check  --role ROLE [--files A,B | --stdin | --from REF [--to REF]] [--json]
                  Bounded merge authority (decision 010). Stricter than verify:
                  every file must be in auto_merge_paths, not just allowed_paths.
                  Status: merge_allowed | merge_blocked.
  validate [--json]
                  Check agent-roles.riido.json against agent-roles.schema.json
                  (mirrored by ValidateRoles). Exit 1 on schema violations.
  list
  show     --role ROLE [--json]`)
}
