package docfresh

import (
	"strings"
	"testing"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/decision"
)

func makeRec(id string, guards ...string) *decision.Record {
	r := &decision.Record{ID: id, Path: "decisions/2026/06/03/" + id + ".decision.riido.json"}
	for _, g := range guards {
		r.Guards = append(r.Guards, decision.Guard{Kind: "go-tool", Ref: g, Status: "implemented"})
	}
	return r
}

func TestIsAgentVisible(t *testing.T) {
	cases := []struct {
		name   string
		guards []string
		want   bool
	}{
		{"no guards", nil, false},
		{"unrelated tool", []string{"tools/wirelint — scans for wire literals"}, false},
		{"internal/probe match", []string{"internal/probe.NormalizePurpose"}, true},
		{"internal/agentguard match", []string{"internal/agentguard.MergeCheck"}, true},
		{"purpose-banlist match", []string{"purpose-banlist.riido.json seeded entry"}, true},
		{"agent-roles match", []string{"agent-roles.riido.json updated"}, true},
		{"any one match wins", []string{"tools/ssotdeps cross-check", "internal/probe.noiseMarkerReason"}, true},
		{"docfresh self-reference must NOT trigger", []string{"internal/docfresh.Check", "tools/docfresh"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := makeRec("000-test", tc.guards...)
			if got := IsAgentVisible(r); got != tc.want {
				t.Errorf("guards=%v: got %v, want %v", tc.guards, got, tc.want)
			}
		})
	}
}

func TestCheckPasses(t *testing.T) {
	// Two decisions; one agent-visible; AGENT_CONTRACT.md is in the diff.
	added := []*decision.Record{
		makeRec("100-x", "internal/probe.noiseMarkerReason"),
		makeRec("101-y", "tools/wirelint"),
	}
	changed := []string{
		"decisions/2026/06/03/100-x.decision.riido.json",
		"decisions/2026/06/03/101-y.decision.riido.json",
		"AGENT_CONTRACT.md",
		"internal/probe/probe.go",
	}
	if v := Check(added, changed); len(v) != 0 {
		t.Errorf("expected 0 violations when AGENT_CONTRACT.md is touched, got %v", v)
	}
}

func TestCheckFlagsMissingDocUpdate(t *testing.T) {
	added := []*decision.Record{
		makeRec("100-x", "internal/probe.noiseMarkerReason"),
	}
	changed := []string{
		"decisions/2026/06/03/100-x.decision.riido.json",
		"internal/probe/probe.go", // touched but not AGENT_CONTRACT.md
	}
	v := Check(added, changed)
	if len(v) != 1 {
		t.Fatalf("expected 1 violation, got %v", v)
	}
	if v[0].DecisionID != "100-x" {
		t.Errorf("violation should cite 100-x, got %+v", v[0])
	}
	if !strings.Contains(v[0].Error(), "AGENT_CONTRACT.md") {
		t.Errorf("violation message should mention AGENT_CONTRACT.md, got %q", v[0].Error())
	}
	if !strings.Contains(v[0].MatchingGuard, "internal/probe") {
		t.Errorf("violation should cite the matching guard, got %q", v[0].MatchingGuard)
	}
}

func TestCheckIgnoresNonAgentVisibleDecisions(t *testing.T) {
	added := []*decision.Record{
		makeRec("100-x", "tools/ssotdeps cross-check"),
		makeRec("101-y", "tools/wirelint"),
	}
	changed := []string{
		"decisions/2026/06/03/100-x.decision.riido.json",
		"decisions/2026/06/03/101-y.decision.riido.json",
		"tools/ssotdeps/main.go",
		// no AGENT_CONTRACT.md, no agent-visible decision
	}
	if v := Check(added, changed); len(v) != 0 {
		t.Errorf("non-agent-visible decisions should not require AGENT_CONTRACT.md, got %v", v)
	}
}

func TestCheckMultipleViolations(t *testing.T) {
	added := []*decision.Record{
		makeRec("100-x", "internal/probe.noiseMarkerReason"),
		makeRec("101-y", "internal/agentguard.MergeCheck"),
		makeRec("102-z", "tools/ssotdeps"), // not agent-visible
	}
	changed := []string{
		"decisions/2026/06/03/100-x.decision.riido.json",
		"decisions/2026/06/03/101-y.decision.riido.json",
		"decisions/2026/06/03/102-z.decision.riido.json",
	}
	v := Check(added, changed)
	if len(v) != 2 {
		t.Errorf("expected 2 violations (100-x + 101-y), got %d: %v", len(v), v)
	}
}

func TestIsDecisionPath(t *testing.T) {
	cases := map[string]bool{
		"decisions/2026/06/03/001-initial-agreement.decision.riido.json": true,
		"decisions/something.decision.riido.json":                        true,
		"decisions/schema.riido.json":                                    false,
		"AGENT_CONTRACT.md":                                              false,
		"tools/decision/main.go":                                         false,
		"":                                                               false,
	}
	for p, want := range cases {
		if got := IsDecisionPath(p); got != want {
			t.Errorf("IsDecisionPath(%q) = %v, want %v", p, got, want)
		}
	}
}
