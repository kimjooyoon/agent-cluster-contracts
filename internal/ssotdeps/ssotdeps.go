// Package ssotdeps loads and verifies the SSOT dependency map. It checks that
// every declared artifact, schema, consumer, and CI workflow actually exists on
// disk relative to the contracts repo root.
//
// Verify has two modes:
//   - ModeFull (default): also checks sibling repos (backend, frontend) when
//     they are present in the parent directory. Used by smart-agent local dev
//     and by full cross-repo CI.
//   - ModeLocal: ignores sibling repos entirely, even if their directories
//     exist. Used by the dumb-agent probe preflight so its baseline answer is
//     deterministic regardless of what happens to be in the parent directory.
package ssotdeps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

type Map struct {
	Version         string             `json:"version"`
	Owner           string             `json:"owner"`
	Description     string             `json:"description"`
	SsotArtifacts   []SsotArtifact     `json:"ssot_artifacts"`
	GenerationLinks []GenerationLink   `json:"generation_links"`
	ConsumptionLinks []ConsumptionLink `json:"consumption_links"`
	CIGates         []CIGate           `json:"ci_gates"`
	Pending         []string           `json:"pending"`
}

type SsotArtifact struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Path        string   `json:"path"`
	Schema      *string  `json:"schema"`
	MirroredIn  []string `json:"mirrored_in,omitempty"`
	OwnedBy     string   `json:"owned_by"`
}

type GenerationLink struct {
	From         string `json:"from"`
	Emitter      string `json:"emitter"`
	To           string `json:"to"`
	ConsumerRepo string `json:"consumer_repo,omitempty"`
	Guard        string `json:"guard,omitempty"`
}

type ConsumptionLink struct {
	SSOT         string `json:"ssot"`
	ConsumerRepo string `json:"consumer_repo"`
	ConsumerPath string `json:"consumer_path"`
	Via          string `json:"via,omitempty"`
	Guard        string `json:"guard,omitempty"`
	// Planned: the consumer file does not exist yet (generator hasn't run in
	// the sibling repo). Verify skips the existence check so the dep map can
	// record the dependency before the file lands.
	Planned bool `json:"planned,omitempty"`
}

type CIGate struct {
	Repo     string   `json:"repo"`
	Workflow string   `json:"workflow"`
	Verifies []string `json:"verifies"`
	// Planned means this gate is declared but not yet wired up. Verify skips
	// the file-existence check for planned gates so the dep map can record
	// intent without false-positive failures. Flip to false when the file
	// lands.
	Planned bool `json:"planned,omitempty"`
}

// Load reads root/ssot-dependency-map.riido.json.
func Load(root string) (*Map, error) {
	m := &Map{}
	if err := jsonutil.ReadFile(filepath.Join(root, "ssot-dependency-map.riido.json"), m); err != nil {
		return nil, err
	}
	return m, nil
}

// Mode controls how much of the dep map is checked.
type Mode int

const (
	// ModeFull verifies everything, including sibling-repo consumer paths and
	// CI gates in sibling repos when those repos are present locally. This is
	// the default and what cross-repo CI uses.
	ModeFull Mode = iota
	// ModeLocal verifies only contracts-repo-owned artifacts and gates. It
	// never reads sibling-repo state, even if backend/ or frontend/ are
	// present. This is what the dumb-agent probe preflight uses so the
	// baseline answer cannot be polluted by an incomplete sibling checkout.
	ModeLocal
)

func (m Mode) String() string {
	switch m {
	case ModeLocal:
		return "local"
	default:
		return "full"
	}
}

// Verify checks that referenced files exist. Returns one error per failure
// (empty slice means OK). See Mode for the cross-repo behavior.
func Verify(root string, m *Map, mode Mode) []error {
	var errs []error
	ids := map[string]bool{}
	for i, a := range m.SsotArtifacts {
		ids[a.ID] = true
		if a.Path == "" {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d].path: required", i))
			continue
		}
		if err := mustExist(root, a.Path); err != nil {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (id=%s): %w", i, a.ID, err))
		}
		if a.Schema != nil && *a.Schema != "" {
			if err := mustExist(root, *a.Schema); err != nil {
				errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (id=%s) schema: %w", i, a.ID, err))
			}
		}
		for _, m := range a.MirroredIn {
			if err := mustExist(root, m); err != nil {
				errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (id=%s) mirror: %w", i, a.ID, err))
			}
		}
	}
	for i, l := range m.ConsumptionLinks {
		if !ids[l.SSOT] {
			errs = append(errs, fmt.Errorf("consumption_links[%d].ssot %q: not in ssot_artifacts", i, l.SSOT))
		}
		if l.Planned {
			continue
		}
		if l.ConsumerRepo == "agent-cluster-contracts" {
			if err := mustExist(root, l.ConsumerPath); err != nil {
				errs = append(errs, fmt.Errorf("consumption_links[%d] consumer_path: %w", i, err))
			}
			continue
		}
		// Sibling repo. In ModeLocal we never look at siblings — that's the
		// point of local mode: deterministic baseline regardless of sibling
		// checkout state.
		if mode == ModeLocal {
			continue
		}
		sibling := siblingRoot(root, l.ConsumerRepo)
		if sibling == "" {
			continue
		}
		if _, err := os.Stat(sibling); err != nil {
			continue // sibling not checked out; skip
		}
		if err := mustExist(sibling, l.ConsumerPath); err != nil {
			errs = append(errs, fmt.Errorf("consumption_links[%d] consumer_path in sibling %s: %w", i, l.ConsumerRepo, err))
		}
	}
	for i, l := range m.GenerationLinks {
		if l.From != "" {
			if err := mustExist(root, l.From); err != nil {
				errs = append(errs, fmt.Errorf("generation_links[%d].from: %w", i, err))
			}
		}
	}
	for i, g := range m.CIGates {
		if g.Planned {
			continue
		}
		base := root
		if g.Repo != "agent-cluster-contracts" {
			// Skip sibling CI gates entirely in local mode.
			if mode == ModeLocal {
				continue
			}
			base = siblingRoot(root, g.Repo)
			if base == "" {
				continue
			}
			if _, err := os.Stat(base); err != nil {
				continue
			}
		}
		if err := mustExist(base, g.Workflow); err != nil {
			errs = append(errs, fmt.Errorf("ci_gates[%d] workflow: %w", i, err))
		}
	}
	return errs
}

func mustExist(base, rel string) error {
	p := filepath.Join(base, rel)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("path %q does not exist (resolved to %s)", rel, p)
		}
		return err
	}
	return nil
}

// siblingRoot maps "agent-cluster-backend" → "<parent-of-root>/backend" and
// "agent-cluster-frontend" → "<parent-of-root>/frontend". The local checkout
// uses short dir names per the bootstrap convention.
func siblingRoot(contractsRoot, repo string) string {
	parent := filepath.Dir(contractsRoot)
	switch repo {
	case "agent-cluster-backend":
		return filepath.Join(parent, "backend")
	case "agent-cluster-frontend":
		return filepath.Join(parent, "frontend")
	case "agent-cluster-contracts":
		return contractsRoot
	}
	return ""
}
