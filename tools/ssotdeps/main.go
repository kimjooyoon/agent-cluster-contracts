// ssotdeps verify: load ssot-dependency-map.riido.json and check that every
// referenced path exists. Exits non-zero on any failure.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/findroot"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/ssotdeps"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "verify":
		cmdVerify(os.Args[2:])
	case "show":
		cmdShow(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func cmdVerify(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "JSON output")
	fs.Parse(args)
	root, err := findroot.FromCWD()
	if err != nil {
		die(err)
	}
	m, err := ssotdeps.Load(root)
	if err != nil {
		die(err)
	}
	errs := ssotdeps.Verify(root, m)
	if *asJSON {
		out := map[string]any{"ok": len(errs) == 0, "errors": errMsgs(errs), "root": root}
		json.NewEncoder(os.Stdout).Encode(out)
	} else {
		if len(errs) == 0 {
			fmt.Printf("ssotdeps verify: OK (%d artifacts, %d consumption links, %d CI gates)\n",
				len(m.SsotArtifacts), len(m.ConsumptionLinks), len(m.CIGates))
		} else {
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "FAIL:", e)
			}
		}
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
}

func cmdShow(args []string) {
	root, err := findroot.FromCWD()
	if err != nil {
		die(err)
	}
	m, err := ssotdeps.Load(root)
	if err != nil {
		die(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(m)
}

func errMsgs(errs []error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Error()
	}
	return out
}

func usage() {
	fmt.Fprintln(os.Stderr, `ssotdeps — verify the SSOT dependency map

Commands:
  verify [--json]   Check that every referenced path exists. Exit 1 on failure.
  show              Print the loaded dep map as JSON.`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
