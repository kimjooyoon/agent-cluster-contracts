// decision: list, explain, create, validate, supersede decision records.
//
// Usage:
//   decision list [--status accepted|proposed|superseded|rejected] [--json]
//   decision explain <id> [--json]
//   decision create --id NNN-slug --title "..." --status proposed --source top_down|bottom_up --owner gh-login --area area1,area2 --repo R1,R2 --ssot R --example "..." --evidence kind=ref,...
//   decision validate                  (validate every record)
//   decision supersede --old <id> --new <id>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/decision"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	root, err := findroot.FromCWD()
	if err != nil {
		die(err)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "list":
		cmdList(root, args)
	case "explain":
		cmdExplain(root, args)
	case "create":
		cmdCreate(root, args)
	case "validate":
		cmdValidate(root)
	case "supersede":
		cmdSupersede(root, args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", cmd)
		usage()
		os.Exit(2)
	}
}

func cmdList(root string, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	status := fs.String("status", "", "filter by status")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	all, err := decision.LoadAll(root)
	if err != nil {
		die(err)
	}
	var filtered []*decision.Record
	for _, r := range all {
		if *status != "" && r.Status != *status {
			continue
		}
		filtered = append(filtered, r)
	}
	if *asJSON {
		writeJSON(filtered)
		return
	}
	if len(filtered) == 0 {
		fmt.Println("(no decision records found)")
		return
	}
	for _, r := range filtered {
		flag := ""
		if r.Weak {
			flag = " [weak]"
		}
		fmt.Printf("%s  %-12s  %s%s\n", r.ID, r.Status, r.Title, flag)
	}
}

func cmdExplain(root string, args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	if fs.NArg() != 1 {
		die(fmt.Errorf("usage: decision explain <id>"))
	}
	id := fs.Arg(0)
	all, err := decision.LoadAll(root)
	if err != nil {
		die(err)
	}
	for _, r := range all {
		if r.ID == id {
			if *asJSON {
				writeJSON(r)
			} else {
				printRecord(r)
			}
			return
		}
	}
	die(fmt.Errorf("decision %q not found", id))
}

func cmdCreate(root string, args []string) {
	fs := flag.NewFlagSet("create", flag.ExitOnError)
	id := fs.String("id", "", "id (NNN-slug)")
	title := fs.String("title", "", "title")
	status := fs.String("status", "proposed", "status")
	source := fs.String("source", "", "top_down|bottom_up|generated|imported")
	owner := fs.String("owner", "", "owner github login")
	areasCSV := fs.String("area", "", "comma-separated areas")
	bcCSV := fs.String("bc", "", "comma-separated bounded contexts")
	reposCSV := fs.String("repo", "agent-cluster-contracts", "comma-separated affected repos")
	ssotOwner := fs.String("ssot", "agent-cluster-contracts", "SSOT owner repo")
	exampleCSV := fs.String("example", "", "comma-separated example strings (at least one required)")
	evidenceCSV := fs.String("evidence", "", "comma-separated evidence entries kind=ref")
	weak := fs.Bool("weak", false, "weak decision (does not block merges)")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "created_at date YYYY-MM-DD")
	fs.Parse(args)
	if *id == "" || *title == "" || *source == "" || *owner == "" || *exampleCSV == "" || *evidenceCSV == "" {
		die(fmt.Errorf("required: --id --title --source --owner --example --evidence"))
	}
	r := &decision.Record{
		ID:            *id,
		Title:         *title,
		Owner:         *owner,
		Status:        *status,
		Weak:          *weak,
		Source:        *source,
		AffectedRepos: splitCSV(*reposCSV),
		SsotOwner:     *ssotOwner,
		Examples:      splitCSV(*exampleCSV),
		CreatedAt:     *date,
		Scope: decision.Scope{
			BoundedContexts: splitCSV(*bcCSV),
			Areas:           splitCSV(*areasCSV),
		},
	}
	for _, ev := range splitCSV(*evidenceCSV) {
		kv := strings.SplitN(ev, "=", 2)
		if len(kv) != 2 {
			die(fmt.Errorf("evidence entry %q: expected kind=ref", ev))
		}
		r.Evidence = append(r.Evidence, decision.Evidence{Kind: kv[0], Ref: kv[1]})
	}
	if *status == "accepted" {
		now := *date
		r.AcceptedAt = &now
	}
	if errs := decision.Validate(r); len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "invalid:", e)
		}
		os.Exit(1)
	}
	path, err := decision.PathFor(root, *date, *id)
	if err != nil {
		die(err)
	}
	if _, err := os.Stat(path); err == nil {
		die(fmt.Errorf("decision file already exists: %s", path))
	}
	if err := decision.Save(r, path); err != nil {
		die(err)
	}
	fmt.Println("wrote:", path)
}

func cmdValidate(root string) {
	all, err := decision.LoadAll(root)
	if err != nil {
		die(err)
	}
	fail := false
	for _, r := range all {
		errs := decision.Validate(r)
		if len(errs) > 0 {
			fail = true
			fmt.Fprintf(os.Stderr, "%s: %d error(s)\n", r.Path, len(errs))
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "  -", e)
			}
		}
	}
	if fail {
		os.Exit(1)
	}
	fmt.Printf("validated %d record(s): OK\n", len(all))
}

func cmdSupersede(root string, args []string) {
	fs := flag.NewFlagSet("supersede", flag.ExitOnError)
	old := fs.String("old", "", "old decision id")
	newID := fs.String("new", "", "new decision id (must already exist)")
	fs.Parse(args)
	if *old == "" || *newID == "" {
		die(fmt.Errorf("usage: decision supersede --old <id> --new <id>"))
	}
	all, err := decision.LoadAll(root)
	if err != nil {
		die(err)
	}
	var oldR, newR *decision.Record
	for _, r := range all {
		if r.ID == *old {
			oldR = r
		}
		if r.ID == *newID {
			newR = r
		}
	}
	if oldR == nil {
		die(fmt.Errorf("--old %q: not found", *old))
	}
	if newR == nil {
		die(fmt.Errorf("--new %q: not found (create it first with `decision create`)", *newID))
	}
	oldR.Status = "superseded"
	oldR.SupersededBy = newID
	newR.Supersedes = append(newR.Supersedes, *old)
	if err := decision.Save(oldR, oldR.Path); err != nil {
		die(err)
	}
	if err := decision.Save(newR, newR.Path); err != nil {
		die(err)
	}
	fmt.Printf("superseded %s → %s\n", *old, *newID)
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

func writeJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		die(err)
	}
}

func printRecord(r *decision.Record) {
	fmt.Printf("ID:      %s\n", r.ID)
	fmt.Printf("Title:   %s\n", r.Title)
	fmt.Printf("Status:  %s\n", r.Status)
	if r.Weak {
		fmt.Println("Weak:    true")
	}
	fmt.Printf("Owner:   %s\n", r.Owner)
	fmt.Printf("Source:  %s\n", r.Source)
	fmt.Printf("Created: %s\n", r.CreatedAt)
	fmt.Printf("Areas:   %s\n", strings.Join(r.Scope.Areas, ", "))
	fmt.Printf("Repos:   %s\n", strings.Join(r.AffectedRepos, ", "))
	fmt.Printf("SSOT:    %s\n", r.SsotOwner)
	if len(r.Evidence) > 0 {
		fmt.Println("Evidence:")
		for _, e := range r.Evidence {
			fmt.Printf("  - %s: %s\n", e.Kind, e.Ref)
		}
	}
	if len(r.Examples) > 0 {
		fmt.Println("Examples:")
		for _, ex := range r.Examples {
			fmt.Printf("  - %s\n", ex)
		}
	}
	if len(r.Guards) > 0 {
		fmt.Println("Guards:")
		for _, g := range r.Guards {
			fmt.Printf("  - %s: %s (%s)\n", g.Kind, g.Ref, g.Status)
		}
	}
	if len(r.Supersedes) > 0 {
		fmt.Println("Supersedes:", strings.Join(r.Supersedes, ", "))
	}
	if r.SupersededBy != nil {
		fmt.Println("Superseded by:", *r.SupersededBy)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `decision — list, explain, create, validate, supersede decision records

Commands:
  list       [--status STATUS] [--json]
  explain    <id> [--json]
  create     --id NNN-slug --title "..." --source top_down|bottom_up --owner gh-login
             --example "..." --evidence kind=ref [--status proposed] [--area a,b]
             [--repo R1,R2] [--ssot R] [--weak] [--date YYYY-MM-DD]
  validate
  supersede  --old <id> --new <id>`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
