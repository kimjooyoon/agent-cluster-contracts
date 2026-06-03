// ssotdeps verify: load ssot-dependency-map.riido.json and check that every
// referenced path exists. Exits non-zero on any failure.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/conceptmap"
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
	case "validate":
		cmdValidate(os.Args[2:])
	case "cross-check":
		cmdCrossCheck(os.Args[2:])
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

func cmdValidate(args []string) {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
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
	errs := ssotdeps.ValidateMap(m)
	if *asJSON {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": len(errs) == 0, "errors": msgs})
	} else {
		if len(errs) == 0 {
			fmt.Printf("ssotdeps validate: OK (%d artifacts)\n", len(m.SsotArtifacts))
		} else {
			fmt.Fprintf(os.Stderr, "ssotdeps validate: %d error(s)\n", len(errs))
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "  -", e)
			}
		}
	}
	if len(errs) > 0 {
		os.Exit(1)
	}
}

func cmdCrossCheck(args []string) {
	fs := flag.NewFlagSet("cross-check", flag.ExitOnError)
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
	cm, err := conceptmap.Load(root)
	if err != nil {
		die(err)
	}
	errs := ssotdeps.CrossCheck(m, cm)
	if *asJSON {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": len(errs) == 0, "errors": errMsgs(errs)})
	} else {
		if len(errs) == 0 {
			fmt.Printf("ssotdeps cross-check: OK (%d pending entries, none reference implemented constraints)\n", len(m.Pending))
		} else {
			fmt.Fprintf(os.Stderr, "ssotdeps cross-check: %d stale reference(s)\n", len(errs))
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "  -", e)
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
  verify   [--mode local|full] [--json]
                    Check that every referenced path exists.
                    --mode full (default) also checks sibling backend/frontend
                    consumer paths and CI gates when those repos are present.
                    --mode local skips all sibling-repo checks; use this for
                    the dumb-agent probe baseline. Exit 1 on failure.
  validate [--json]
                    Check ssot-dependency-map.riido.json against
                    ssot-dependency-map.schema.json (mirrored by ValidateMap).
                    Catches typos and structural issues that JSON parse alone
                    silently accepts. Exit 1 on schema violations.
  cross-check [--json]
                    Cross-check pending[] against concept-map: any pending entry
                    that mentions an already-implemented constraint id (C-XXX)
                    is stale and must be removed. Decision 022. Exit 1 on
                    stale references.
  show              Print the loaded dep map as JSON.`)
}

func die(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
