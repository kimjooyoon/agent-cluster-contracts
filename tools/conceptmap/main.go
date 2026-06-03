// conceptmap verify / query: validate the concept map and query it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/conceptmap"
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
	switch os.Args[1] {
	case "verify":
		cmdVerify(root)
	case "query":
		cmdQuery(root, os.Args[2:])
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

func cmdVerify(root string) {
	m, err := conceptmap.Load(root)
	if err != nil {
		die(err)
	}
	errs := conceptmap.Validate(m)
	if len(errs) > 0 {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "FAIL:", e)
		}
		os.Exit(1)
	}
	fmt.Printf("conceptmap verify: OK (%d concepts, %d relationships, %d constraints)\n",
		len(m.Concepts), len(m.Relationships), len(m.Constraints))
}

func cmdQuery(root string, args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	if fs.NArg() != 1 {
		die(fmt.Errorf("usage: conceptmap query [--json] <concept-name>"))
	}
	name := fs.Arg(0)
	m, err := conceptmap.Load(root)
	if err != nil {
		die(err)
	}
	c, rels, cons := m.Query(name)
	if *asJSON {
		out := map[string]any{"concept": c, "relationships": rels, "constraints": cons}
		json.NewEncoder(os.Stdout).Encode(out)
		return
	}
	if c == nil && len(rels) == 0 && len(cons) == 0 {
		fmt.Println("(no matches)")
		return
	}
	if c != nil {
		fmt.Printf("Concept: %s (%s, owned by %s)\n", c.Name, c.Kind, c.Owner)
		fmt.Printf("  %s\n", c.Description)
	}
	if len(rels) > 0 {
		fmt.Println("Relationships:")
		for _, r := range rels {
			fmt.Printf("  - %s %s %s   (e.g. %s)\n", r.From, r.Kind, r.To, r.Example)
		}
	}
	if len(cons) > 0 {
		fmt.Println("Constraints:")
		for _, k := range cons {
			fmt.Printf("  - %s: %s\n", k.ID, k.Rule)
		}
	}
}

func cmdList(root string, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	m, err := conceptmap.Load(root)
	if err != nil {
		die(err)
	}
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(m)
		return
	}
	fmt.Println("Concepts:")
	for _, c := range m.Concepts {
		fmt.Printf("  %-22s %-10s %s\n", c.Name, c.Kind, c.Owner)
	}
	fmt.Println("Constraints:")
	for _, k := range m.Constraints {
		fmt.Printf("  %s  %s\n", k.ID, k.Rule)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `conceptmap — verify / query the concept map

Commands:
  verify              Validate concept-map/concept-map.riido.json. Exit 1 on failure.
  query <name>        Print the concept, related relationships, and constraints.
  list [--json]       List all concepts and constraints.`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
