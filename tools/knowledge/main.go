// knowledge: ingest source files into an inverted index, query it, append
// records, supersede records. Output formats: human and --json.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/knowledge"
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
	switch os.Args[1] {
	case "ingest":
		cmdIngest(root, os.Args[2:])
	case "query":
		cmdQuery(root, os.Args[2:])
	case "record":
		cmdRecord(root, os.Args[2:])
	case "supersede":
		cmdSupersede(root, os.Args[2:])
	case "list":
		cmdList(root, os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func cmdIngest(root string, args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	extraCSV := fs.String("extra", "", "comma-separated additional roots to index (sibling repos, e.g. ../backend,../frontend)")
	fs.Parse(args)
	roots := []string{root}
	for _, p := range splitCSV(*extraCSV) {
		if _, err := os.Stat(p); err == nil {
			roots = append(roots, p)
		}
	}
	idx, err := knowledge.Build(roots)
	if err != nil {
		die(err)
	}
	if err := knowledge.Save(root, idx); err != nil {
		die(err)
	}
	fmt.Printf("indexed %d files from %d root(s); %d tokens\n", len(idx.Files), len(roots), len(idx.Tokens))
}

func cmdQuery(root string, args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	q := fs.String("q", "", "query string")
	top := fs.Int("top", 10, "top N results")
	agent := fs.String("agent", "", "claude|codex (logged; does not change ranking)")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	if *q == "" {
		die(fmt.Errorf("--q required"))
	}
	idx, err := knowledge.LoadIndex(root)
	if err != nil {
		die(fmt.Errorf("load index: %w (try: knowledge ingest)", err))
	}
	results := idx.Query(*q, *top)
	if *asJSON {
		out := map[string]any{
			"agent":   *agent,
			"query":   *q,
			"results": results,
		}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}
	if len(results) == 0 {
		fmt.Println("(no matches)")
		return
	}
	for _, r := range results {
		fmt.Printf("%6.1f  %s  (lines: %v)\n", r.Score, r.Path, r.Lines)
	}
}

func cmdRecord(root string, args []string) {
	fs := flag.NewFlagSet("record", flag.ExitOnError)
	id := fs.String("id", "", "record id (required, e.g. K-2026-06-03-001)")
	kind := fs.String("kind", "note", "note|pr-summary|ci-failure|incident|observation")
	title := fs.String("title", "", "short title (required)")
	body := fs.String("body", "", "free-form body")
	decision := fs.String("decision", "", "related decision id")
	tagsCSV := fs.String("tags", "", "comma-separated tags")
	fs.Parse(args)
	if *id == "" || *title == "" {
		die(fmt.Errorf("--id and --title required"))
	}
	r := knowledge.KnowledgeRecord{
		ID:       *id,
		At:       time.Now().UTC(),
		Kind:     *kind,
		Title:    *title,
		Body:     *body,
		Decision: *decision,
		Tags:     splitCSV(*tagsCSV),
		Status:   "active",
	}
	if err := knowledge.AppendRecord(root, r); err != nil {
		die(err)
	}
	fmt.Println("recorded:", *id)
}

func cmdSupersede(root string, args []string) {
	fs := flag.NewFlagSet("supersede", flag.ExitOnError)
	old := fs.String("old", "", "old record id")
	newID := fs.String("new", "", "new record id")
	note := fs.String("note", "", "reason")
	fs.Parse(args)
	if *old == "" || *newID == "" {
		die(fmt.Errorf("--old and --new required"))
	}
	if err := knowledge.Supersede(root, *old, *newID, *note); err != nil {
		die(err)
	}
	fmt.Printf("superseded %s → %s\n", *old, *newID)
}

func cmdList(root string, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	all, err := knowledge.LoadRecords(root)
	if err != nil {
		die(err)
	}
	live := knowledge.Fold(all)
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(live)
		return
	}
	if len(live) == 0 {
		fmt.Println("(no records)")
		return
	}
	for _, r := range live {
		fmt.Printf("%s  %s  %-15s  %s\n", r.At.Format(time.RFC3339), r.ID, r.Kind, r.Title)
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

func usage() {
	fmt.Fprintln(os.Stderr, `knowledge — file-based knowledge index + record log

Commands:
  ingest    [--extra ../backend,../frontend]
  query     --q "..." [--top N] [--agent claude|codex] [--json]
  record    --id ID --title "..." [--kind ...] [--body ...] [--decision NNN-slug] [--tags ...]
  supersede --old ID --new ID [--note ...]
  list      [--json]`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
