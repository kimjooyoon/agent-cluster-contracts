// devpattern: select top_down|bottom_up randomly, append event, list, summary.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/devpattern"
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
	case "select":
		cmdSelect(root, os.Args[2:])
	case "list":
		cmdList(root, os.Args[2:])
	case "summary":
		cmdSummary(root)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func cmdSelect(root string, args []string) {
	fs := flag.NewFlagSet("select", flag.ExitOnError)
	wi := fs.String("work-item", "", "work item id (required)")
	override := fs.String("override", "", "force top_down or bottom_up (records override=true)")
	note := fs.String("note", "", "free-form note")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	if *wi == "" {
		die(fmt.Errorf("--work-item required"))
	}
	var source string
	var isOverride bool
	if *override != "" {
		if *override != devpattern.SourceTopDown && *override != devpattern.SourceBottomUp {
			die(fmt.Errorf("--override must be top_down or bottom_up"))
		}
		source = *override
		isOverride = true
	} else {
		s, err := devpattern.Select()
		if err != nil {
			die(err)
		}
		source = s
	}
	e := devpattern.Event{
		WorkItem: *wi,
		Source:   source,
		At:       time.Now().UTC(),
		Note:     *note,
		Override: isOverride,
	}
	if err := devpattern.Append(root, e); err != nil {
		die(err)
	}
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(e)
		return
	}
	flagStr := ""
	if isOverride {
		flagStr = " [override]"
	}
	fmt.Printf("%s: %s%s\n", *wi, source, flagStr)
}

func cmdList(root string, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	wi := fs.String("work-item", "", "filter by work item")
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	all, err := devpattern.Load(root)
	if err != nil {
		die(err)
	}
	var filtered []devpattern.Event
	for _, e := range all {
		if *wi != "" && e.WorkItem != *wi {
			continue
		}
		filtered = append(filtered, e)
	}
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(filtered)
		return
	}
	for _, e := range filtered {
		flagStr := ""
		if e.Override {
			flagStr = " [override]"
		}
		fmt.Printf("%s  %-12s  %s%s\n", e.At.Format(time.RFC3339), e.WorkItem, e.Source, flagStr)
	}
}

func cmdSummary(root string) {
	all, err := devpattern.Load(root)
	if err != nil {
		die(err)
	}
	counts := map[string]int{}
	overrides := 0
	for _, e := range all {
		counts[e.Source]++
		if e.Override {
			overrides++
		}
	}
	fmt.Printf("events: %d   top_down: %d   bottom_up: %d   overrides: %d\n",
		len(all), counts[devpattern.SourceTopDown], counts[devpattern.SourceBottomUp], overrides)
}

func usage() {
	fmt.Fprintln(os.Stderr, `devpattern — record top_down vs bottom_up source pattern per work item

Commands:
  select --work-item ID [--override top_down|bottom_up] [--note "..."] [--json]
  list   [--work-item ID] [--json]
  summary`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
