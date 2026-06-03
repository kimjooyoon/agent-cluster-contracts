// Package docfresh implements the AGENT_CONTRACT.md freshness guard
// introduced by decision 025.
//
// Recurring pattern observed across D015..D023: code-change decisions
// introduce rules dumb-agent must obey; AGENT_CONTRACT.md (the doc
// dumb-agent reads first) lags behind for several decisions; a later
// catch-up decision (D021, D024) re-syncs the doc. Two of 24 decisions
// were pure doc catch-ups by the time D024 landed.
//
// docfresh closes the loop: a PR that adds a new decision record whose
// guards[].ref points at a dumb-agent-visible rule must also touch
// AGENT_CONTRACT.md in the same diff. A one-line acknowledgement is
// enough; what we're enforcing is that the author has at least visited
// the doc to decide whether an update is needed.
package docfresh

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/decision"
)

// AgentContractPath is the canonical relative path of the file the
// guard requires the PR to touch.
const AgentContractPath = "AGENT_CONTRACT.md"

// agentVisibleGuardRe matches guards[].ref strings whose referenced
// rule lives in the dumb-agent's primary read surface. If any guard in
// a newly added decision matches this pattern, AGENT_CONTRACT.md must
// be in the PR diff.
//
// Substrings chosen because they appear in real D015..D024 guards:
//   - internal/probe — fixture purpose rules + noise-marker + banlist
//   - internal/agentguard — role allowlist / merge predicate
//   - purpose-banlist — the SSOT banlist itself
//   - agent-roles — role definitions
//
// Adding a new agent-visible surface (e.g. a new SSOT the dumb-agent
// reads at preflight time) requires extending this regex via a
// superseding decision.
var agentVisibleGuardRe = regexp.MustCompile(
	`internal/probe|internal/agentguard|purpose-banlist|agent-roles`,
)

// IsAgentVisible returns true if any guard in the record references a
// dumb-agent-visible rule surface.
func IsAgentVisible(r *decision.Record) bool {
	if r == nil {
		return false
	}
	for _, g := range r.Guards {
		if agentVisibleGuardRe.MatchString(g.Ref) {
			return true
		}
	}
	return false
}

// Violation describes one decision that requires an AGENT_CONTRACT.md
// update but did not get one in this PR.
type Violation struct {
	DecisionPath  string
	DecisionID    string
	MatchingGuard string
}

func (v Violation) Error() string {
	return fmt.Sprintf(
		"decision %s (%s) has a guard referencing a dumb-agent-visible rule (%q) but the PR does not touch %s. Add a one-line acknowledgement to %s, or — if no doc change is warranted — say so explicitly in the decision's rationale and add a marker comment in %s",
		v.DecisionID, v.DecisionPath, v.MatchingGuard, AgentContractPath, AgentContractPath, AgentContractPath,
	)
}

// Check inspects a list of decision records that were ADDED in a PR
// (not the full repo set; only what this PR introduces) and the list
// of all file paths the PR touched. Returns one Violation per
// agent-visible decision whose PR does not also touch AGENT_CONTRACT.md.
func Check(addedDecisions []*decision.Record, changedFiles []string) []Violation {
	touchesAgentContract := false
	for _, f := range changedFiles {
		if filepath.ToSlash(f) == AgentContractPath {
			touchesAgentContract = true
			break
		}
	}
	if touchesAgentContract {
		return nil
	}
	var v []Violation
	for _, r := range addedDecisions {
		if !IsAgentVisible(r) {
			continue
		}
		// Find the specific guard ref that triggered the rule so the
		// violation message is actionable.
		match := ""
		for _, g := range r.Guards {
			if agentVisibleGuardRe.MatchString(g.Ref) {
				match = g.Ref
				break
			}
		}
		v = append(v, Violation{
			DecisionPath:  r.Path,
			DecisionID:    r.ID,
			MatchingGuard: match,
		})
	}
	return v
}

// IsDecisionPath reports whether p looks like a decision record path
// (under decisions/ and ending in .decision.riido.json). Used by the
// CLI when filtering the diff.
func IsDecisionPath(p string) bool {
	s := filepath.ToSlash(p)
	return strings.HasPrefix(s, "decisions/") && strings.HasSuffix(s, ".decision.riido.json")
}
