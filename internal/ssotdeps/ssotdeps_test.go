package ssotdeps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/conceptmap"
)

func TestModeLocalSkipsSiblingConsumers(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root) // sibling lookup uses parent dir
	// Make a sibling backend dir that is MISSING the required consumer path,
	// so full mode would fail. Local mode must skip it.
	backend := filepath.Join(parent, "backend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(backend) })

	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "ir-x", Path: "ir.json", OwnedBy: "agent-cluster-contracts"},
		},
		ConsumptionLinks: []ConsumptionLink{
			{
				SSOT:         "ir-x",
				ConsumerRepo: "agent-cluster-backend",
				ConsumerPath: "internal/contracts/missing.go", // does not exist
			},
		},
	}
	// Make the ssot artifact exist so the only failure source is the sibling.
	if err := os.WriteFile(filepath.Join(root, "ir.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	if errs := Verify(root, m, ModeFull); len(errs) == 0 {
		t.Errorf("full mode: expected failure for missing sibling consumer, got none")
	}
	if errs := Verify(root, m, ModeLocal); len(errs) != 0 {
		t.Errorf("local mode: expected no errors for missing sibling consumer, got %v", errs)
	}
}

func TestModeLocalSkipsSiblingCIGate(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)
	frontend := filepath.Join(parent, "frontend")
	if err := os.MkdirAll(frontend, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(frontend) })

	m := &Map{
		CIGates: []CIGate{
			{Repo: "agent-cluster-frontend", Workflow: ".github/workflows/missing.yml"},
		},
	}
	if errs := Verify(root, m, ModeFull); len(errs) == 0 {
		t.Errorf("full mode: expected failure for missing sibling workflow, got none")
	}
	if errs := Verify(root, m, ModeLocal); len(errs) != 0 {
		t.Errorf("local mode: expected no errors for missing sibling workflow, got %v", errs)
	}
}

func TestBothModesFailOnLocalArtifact(t *testing.T) {
	root := t.TempDir()
	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "x", Path: "nope.json", OwnedBy: "agent-cluster-contracts"},
		},
	}
	for _, mode := range []Mode{ModeLocal, ModeFull} {
		errs := Verify(root, m, mode)
		if len(errs) == 0 {
			t.Errorf("mode=%s: expected failure for missing local artifact", mode)
		}
		// Sanity: the error mentions the missing path.
		found := false
		for _, e := range errs {
			if strings.Contains(e.Error(), "nope.json") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("mode=%s: error should mention nope.json, got %v", mode, errs)
		}
	}
}

func TestCrossCheckFlagsImplementedPending(t *testing.T) {
	impl := "implemented"
	pend := "pending"
	cm := &conceptmap.Map{
		Constraints: []conceptmap.Constraint{
			{ID: "C-001", GuardCandidate: &conceptmap.GuardCand{Tool: "vocablint", Status: impl}},
			{ID: "C-099", GuardCandidate: &conceptmap.GuardCand{Tool: "future", Status: pend}},
		},
	}
	cases := []struct {
		name     string
		pending  []string
		wantHits int
	}{
		{"empty", nil, 0},
		{"no constraint refs", []string{"Generation links for later product slice."}, 0},
		{"references implemented constraint", []string{"Cross-repo vocab scan (constraint C-001 enforcement)."}, 1},
		{"references still-pending constraint is OK", []string{"Future work on C-099."}, 0},
		{"mixed entries", []string{
			"Generation links for backend GraphQL schema and frontend client.",
			"Cross-repo vocab scan (constraint C-001 enforcement).",
			"Future work on C-099.",
		}, 1},
		{"multiple stale refs in one entry counted independently", []string{"C-001 and C-001 mentioned twice."}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := &Map{Pending: tc.pending}
			errs := CrossCheck(m, cm)
			if len(errs) != tc.wantHits {
				t.Errorf("CrossCheck: got %d errors, want %d. errors=%v", len(errs), tc.wantHits, errs)
			}
		})
	}
}

func TestCrossCheckTolerantOfNilInputs(t *testing.T) {
	if errs := CrossCheck(nil, nil); len(errs) != 0 {
		t.Errorf("nil inputs: want 0 errors, got %v", errs)
	}
	if errs := CrossCheck(&Map{Pending: []string{"C-001"}}, nil); len(errs) != 0 {
		t.Errorf("nil concept map: want 0 errors (cannot decide), got %v", errs)
	}
	if errs := CrossCheck(nil, &conceptmap.Map{}); len(errs) != 0 {
		t.Errorf("nil dep map: want 0 errors, got %v", errs)
	}
}

func TestModeString(t *testing.T) {
	if ModeLocal.String() != "local" {
		t.Errorf("ModeLocal.String() = %q, want local", ModeLocal.String())
	}
	if ModeFull.String() != "full" {
		t.Errorf("ModeFull.String() = %q, want full", ModeFull.String())
	}
}
