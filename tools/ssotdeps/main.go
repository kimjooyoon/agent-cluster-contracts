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
	modeStr := fs.String("mode", "full", "local | full — local skips sibling-repo checks even when siblings are checked out")
	fs.Parse(args)
	var mode ssotdeps.Mode
	switch *modeStr {
	case "local":
		mode = ssotdeps.ModeLocal
	case "full", "":
		mode = ssotdeps.ModeFull
	default:
		fmt.Fprintln(os.Stderr, "--mode must be local or full")
		os.Exit(2)
	}
	root, err := findroot.FromCWD()
	if err != nil {
		die(err)
	}
	m, err := ssotdeps.Load(root)
	if err != nil {
		die(err)
	}
	errs := ssotdeps.Verify(root, m, mode)
	if *asJSON {
		out := map[string]any{"ok": len(errs) == 0, "mode": mode.String(), "errors": errMsgs(errs), "root": root}
		json.NewEncoder(os.Stdout).Encode(out)
	} else {
		if len(errs) == 0 {
			fmt.Printf("ssotdeps verify (mode=%s): OK (%d artifacts, %d consumption links, %d CI gates)\n",
				mode, len(m.SsotArtifacts), len(m.ConsumptionLinks), len(m.CIGates))
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
  verify [--mode local|full] [--json]
                    Check that every referenced path exists.
                    --mode full (default) also checks sibling backend/frontend
                    consumer paths and CI gates when those repos are present.
                    --mode local skips all sibling-repo checks; use this for
                    the dumb-agent probe baseline. Exit 1 on failure.
  show              Print the loaded dep map as JSON.`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
