package ssotdeps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/agentguard"
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

// D032 — VerifyForbiddenSymmetry: every SSOT artifact with a schema must
// have BOTH data and schema covered by dumb-agent forbidden_paths.

func ptr(s string) *string { return &s }

func TestVerifyForbiddenSymmetryAllPaired(t *testing.T) {
	// Both halves explicitly listed → OK.
	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "x", Path: "x.json", Schema: ptr("x.schema.json")},
		},
	}
	roles := &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "dumb-agent", ForbiddenPaths: []string{"x.json", "x.schema.json"}},
	}}
	if errs := VerifyForbiddenSymmetry(m, roles); len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
}

func TestVerifyForbiddenSymmetryFlagsDataMissing(t *testing.T) {
	// Schema covered via glob, data not covered → asymmetry.
	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "ir-x", Path: "ir/domain/x.ir.json", Schema: ptr("ir/schema/ir.schema.json")},
		},
	}
	roles := &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "dumb-agent", ForbiddenPaths: []string{"ir/schema/**"}},
	}}
	errs := VerifyForbiddenSymmetry(m, roles)
	if len(errs) != 1 {
		t.Fatalf("expected 1 asymmetry, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "ir-x") || !strings.Contains(errs[0].Error(), "data file") {
		t.Errorf("expected error mentioning ir-x and the missing data file, got %q", errs[0].Error())
	}
}

func TestVerifyForbiddenSymmetryFlagsSchemaMissing(t *testing.T) {
	// Data covered, schema not.
	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "agent-roles", Path: "agent-roles.riido.json", Schema: ptr("agent-roles.schema.json")},
		},
	}
	roles := &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "dumb-agent", ForbiddenPaths: []string{"agent-roles.riido.json"}},
	}}
	errs := VerifyForbiddenSymmetry(m, roles)
	if len(errs) != 1 {
		t.Fatalf("expected 1 asymmetry, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "agent-roles") || !strings.Contains(errs[0].Error(), "schema file") {
		t.Errorf("expected error mentioning agent-roles and the missing schema, got %q", errs[0].Error())
	}
}

func TestVerifyForbiddenSymmetryBothUncoveredIsOK(t *testing.T) {
	// Both halves uncovered (neither in forbidden_paths) — not a C-016
	// violation; C-016 only fires when one is covered and the other isn't.
	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "research", Path: "docs/research/source-ledger.riido.json", Schema: ptr("docs/research/schema.json")},
		},
	}
	roles := &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "dumb-agent", ForbiddenPaths: []string{"tools/**"}},
	}}
	if errs := VerifyForbiddenSymmetry(m, roles); len(errs) != 0 {
		t.Errorf("both-uncovered should be OK (not in scope of C-016), got %v", errs)
	}
}

func TestVerifyForbiddenSymmetryArtifactsWithoutSchema(t *testing.T) {
	// Artifacts without a schema field are out of scope.
	m := &Map{
		SsotArtifacts: []SsotArtifact{
			{ID: "doc", Path: "docs/some.md", Schema: nil},
			{ID: "doc2", Path: "docs/other.md", Schema: ptr("")},
		},
	}
	roles := &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "dumb-agent", ForbiddenPaths: []string{"docs/some.md"}},
	}}
	if errs := VerifyForbiddenSymmetry(m, roles); len(errs) != 0 {
		t.Errorf("schemaless artifacts are out of scope, got %v", errs)
	}
}

func TestVerifyForbiddenSymmetryMissingDumbAgentRole(t *testing.T) {
	m := &Map{}
	roles := &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "designer"},
	}}
	errs := VerifyForbiddenSymmetry(m, roles)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "dumb-agent") {
		t.Errorf("expected one error about missing dumb-agent role, got %v", errs)
	}
}

func TestVerifyForbiddenSymmetryNilTolerant(t *testing.T) {
	if errs := VerifyForbiddenSymmetry(nil, nil); len(errs) != 0 {
		t.Errorf("nil/nil: want empty, got %v", errs)
	}
	if errs := VerifyForbiddenSymmetry(&Map{}, nil); len(errs) != 0 {
		t.Errorf("map/nil: want empty, got %v", errs)
	}
}

// D038 — VerifyFileSystemCoverage: every dsl/**/*.lisp and
// ir/domain/*.ir.json on disk must be registered in the dep map.

func TestVerifyFileSystemCoverageAllRegistered(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dsl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ir/domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dsl/foo.lisp"), []byte("(defun x () nil)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ir/domain/bar.ir.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "dsl-foo", Kind: "dsl-source", Path: "dsl/foo.lisp"},
		{ID: "ir-bar", Kind: "ir", Path: "ir/domain/bar.ir.json"},
	}}
	if errs := VerifyFileSystemCoverage(m, root); len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
}

func TestVerifyFileSystemCoverageFlagsUnregisteredDSL(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dsl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dsl/orphan.lisp"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "other", Kind: "dsl-source", Path: "dsl/other.lisp"},
	}}
	errs := VerifyFileSystemCoverage(m, root)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "dsl/orphan.lisp") {
		t.Errorf("violation should cite the orphan, got %q", errs[0].Error())
	}
}

func TestVerifyFileSystemCoverageFlagsUnregisteredIR(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "ir/domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ir/domain/orphan.ir.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{SsotArtifacts: []SsotArtifact{}}
	errs := VerifyFileSystemCoverage(m, root)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "ir/domain/orphan.ir.json") {
		t.Errorf("violation should cite the orphan, got %q", errs[0].Error())
	}
}

func TestVerifyFileSystemCoverageIgnoresUnrelatedFiles(t *testing.T) {
	// Non-.lisp under dsl/ and non-.ir.json under ir/domain/ are out of scope.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "dsl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ir/domain"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "dsl/README.md"), []byte("# notes"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ir/domain/notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{}
	if errs := VerifyFileSystemCoverage(m, root); len(errs) != 0 {
		t.Errorf("non-.lisp / non-.ir.json files must not trip the rule, got %v", errs)
	}
}

func TestVerifyFileSystemCoverageMissingDirsAreOK(t *testing.T) {
	// If dsl/ or ir/domain/ don't exist (fresh repo), no errors.
	root := t.TempDir()
	m := &Map{}
	if errs := VerifyFileSystemCoverage(m, root); len(errs) != 0 {
		t.Errorf("missing dirs should not produce errors, got %v", errs)
	}
}

// D035 — VerifyToolsReadmeCoverage: every verifier/code-generator
// ssot_artifact under tools/ must have a section heading and a path
// mention in tools/README.md.

func writeToolsReadme(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "README.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestVerifyToolsReadmeCoverageAllDocumented(t *testing.T) {
	dir := t.TempDir()
	p := writeToolsReadme(t, dir,
		"# tools\n\n## foo\n\nstuff `tools/foo/main.go`\n\n## bar\n\nmore `tools/bar/main.go`\n")
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "foo-tool", Kind: "verifier", Path: "tools/foo/main.go"},
		{ID: "bar-tool", Kind: "code-generator", Path: "tools/bar/main.go"},
	}}
	if errs := VerifyToolsReadmeCoverage(m, p); len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
}

func TestVerifyToolsReadmeCoverageFlagsMissingHeading(t *testing.T) {
	dir := t.TempDir()
	// Has path mention in a table but no `## foo` section.
	p := writeToolsReadme(t, dir, "# tools\n\nsee `tools/foo/main.go` in some prose\n")
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "foo-tool", Kind: "verifier", Path: "tools/foo/main.go"},
	}}
	errs := VerifyToolsReadmeCoverage(m, p)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "heading=false") {
		t.Errorf("violation should report heading=false, got %q", errs[0].Error())
	}
}

func TestVerifyToolsReadmeCoverageFlagsMissingPath(t *testing.T) {
	dir := t.TempDir()
	// Has heading but no exact path mention.
	p := writeToolsReadme(t, dir, "# tools\n\n## foo\n\ndocs about foo without the source path\n")
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "foo-tool", Kind: "verifier", Path: "tools/foo/main.go"},
	}}
	errs := VerifyToolsReadmeCoverage(m, p)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "path=false") {
		t.Errorf("violation should report path=false, got %q", errs[0].Error())
	}
}

func TestVerifyToolsReadmeCoverageIgnoresNonToolArtifacts(t *testing.T) {
	dir := t.TempDir()
	p := writeToolsReadme(t, dir, "# tools\n")
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "concept-map", Kind: "concept-map", Path: "concept-map/concept-map.riido.json"},
		{ID: "agent-roles", Kind: "agent-roles", Path: "agent-roles.riido.json"},
		{ID: "schema-x", Kind: "schema", Path: "ir/schema/ir.schema.json"},
	}}
	if errs := VerifyToolsReadmeCoverage(m, p); len(errs) != 0 {
		t.Errorf("non-tool artifacts must not trip the rule, got %v", errs)
	}
}

func TestVerifyToolsReadmeCoverageIgnoresVerifiersOutsideToolsDir(t *testing.T) {
	dir := t.TempDir()
	p := writeToolsReadme(t, dir, "# tools\n")
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "external-verifier", Kind: "verifier", Path: "external/verifier/main.go"},
	}}
	if errs := VerifyToolsReadmeCoverage(m, p); len(errs) != 0 {
		t.Errorf("verifiers outside tools/ must not trip the rule, got %v", errs)
	}
}

func TestVerifyToolsReadmeCoverageMissingFileSurfacesError(t *testing.T) {
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "foo-tool", Kind: "verifier", Path: "tools/foo/main.go"},
	}}
	errs := VerifyToolsReadmeCoverage(m, "/nonexistent/README.md")
	if len(errs) == 0 {
		t.Errorf("expected error for missing README")
	}
}

// D034 — VerifyAgentContractMirror: AGENT_CONTRACT.md Allowed/Forbidden
// paths code blocks must match agent-roles dumb-agent role exactly.

func writeAgentContract(t *testing.T, dir string, allowed, forbidden []string) string {
	t.Helper()
	body := "# Agent Contract\n\n## stuff\n\n### Allowed paths (enforced — repo-relative)\n\n```\n" +
		strings.Join(allowed, "\n") + "\n```\n\nMore prose.\n\n### Forbidden paths (enforced — repo-relative)\n\n```\n" +
		strings.Join(forbidden, "\n") + "\n```\n"
	p := filepath.Join(dir, "AGENT_CONTRACT.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func dumbRole(allowed, forbidden []string) *agentguard.Roles {
	return &agentguard.Roles{Roles: []agentguard.Role{
		{ID: "dumb-agent", AllowedPaths: allowed, ForbiddenPaths: forbidden},
	}}
}

func TestVerifyAgentContractMirrorAllMatch(t *testing.T) {
	dir := t.TempDir()
	allowed := []string{"fixtures/positive/**", "reports/guard-candidates/**"}
	forbidden := []string{"tools/**", "decisions/**"}
	p := writeAgentContract(t, dir, allowed, forbidden)
	if errs := VerifyAgentContractMirror(dumbRole(allowed, forbidden), p); len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
}

func TestVerifyAgentContractMirrorFlagsMDMissingEntry(t *testing.T) {
	// JSON has a path that MD lacks.
	dir := t.TempDir()
	p := writeAgentContract(t, dir,
		[]string{"fixtures/positive/**"},
		[]string{"tools/**", "decisions/**"},
	)
	roles := dumbRole(
		[]string{"fixtures/positive/**"},
		[]string{"tools/**", "decisions/**", "ir/domain/**"}, // ir/domain/** missing from MD
	)
	errs := VerifyAgentContractMirror(roles, p)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "ir/domain") || !strings.Contains(errs[0].Error(), "missing from the AGENT_CONTRACT.md") {
		t.Errorf("violation should cite missing-from-md ir/domain, got %q", errs[0].Error())
	}
}

func TestVerifyAgentContractMirrorFlagsJSONMissingEntry(t *testing.T) {
	// MD has a path that JSON lacks.
	dir := t.TempDir()
	p := writeAgentContract(t, dir,
		[]string{"fixtures/positive/**"},
		[]string{"tools/**", "decisions/**", "stray-extra-rule.json"},
	)
	roles := dumbRole(
		[]string{"fixtures/positive/**"},
		[]string{"tools/**", "decisions/**"},
	)
	errs := VerifyAgentContractMirror(roles, p)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "stray-extra-rule.json") || !strings.Contains(errs[0].Error(), "missing from dumb-agent.forbidden_paths") {
		t.Errorf("violation should cite missing-from-json, got %q", errs[0].Error())
	}
}

func TestVerifyAgentContractMirrorBothListsChecked(t *testing.T) {
	// Drift in allowed AND drift in forbidden → 2 violations (one each).
	dir := t.TempDir()
	p := writeAgentContract(t, dir,
		[]string{"fixtures/positive/**"},
		[]string{"tools/**"},
	)
	roles := dumbRole(
		[]string{"fixtures/positive/**", "fuzz/corpus/**"},
		[]string{"tools/**", "decisions/**"},
	)
	errs := VerifyAgentContractMirror(roles, p)
	if len(errs) != 2 {
		t.Errorf("expected 2 violations (one per list), got %d: %v", len(errs), errs)
	}
}

func TestVerifyAgentContractMirrorOrderingIndependent(t *testing.T) {
	dir := t.TempDir()
	// MD lists in different order than JSON — should still match as sets.
	p := writeAgentContract(t, dir,
		[]string{"b/**", "a/**"},
		[]string{"y/**", "x/**"},
	)
	roles := dumbRole(
		[]string{"a/**", "b/**"},
		[]string{"x/**", "y/**"},
	)
	if errs := VerifyAgentContractMirror(roles, p); len(errs) != 0 {
		t.Errorf("ordering shouldn't matter, got %v", errs)
	}
}

func TestVerifyAgentContractMirrorMissingFileSurfacesAsError(t *testing.T) {
	roles := dumbRole(nil, nil)
	errs := VerifyAgentContractMirror(roles, "/nonexistent/AGENT_CONTRACT.md")
	if len(errs) == 0 {
		t.Errorf("expected error for missing AGENT_CONTRACT.md")
	}
}

func TestVerifyAgentContractMirrorNilRolesIsNoOp(t *testing.T) {
	if errs := VerifyAgentContractMirror(nil, "/nonexistent"); len(errs) != 0 {
		t.Errorf("nil roles should produce no errors, got %v", errs)
	}
}

// D033 — VerifyWorkflowToolCoverage: every ./bin/<name> reference in
// .github/workflows/*.yml must be registered as an ssot_artifact.

func TestVerifyWorkflowToolCoverageAllRegistered(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"),
		[]byte("steps:\n  - run: ./bin/foo --json\n  - run: ./bin/bar verify\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "foo-tool", Kind: "verifier", Path: "tools/foo/main.go"},
		{ID: "bar-tool", Kind: "verifier", Path: "tools/bar/main.go"},
	}}
	if errs := VerifyWorkflowToolCoverage(m, wfDir); len(errs) != 0 {
		t.Errorf("expected 0 errors, got %v", errs)
	}
}

func TestVerifyWorkflowToolCoverageFlagsMissing(t *testing.T) {
	root := t.TempDir()
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"),
		[]byte("steps:\n  - run: ./bin/registered --json\n  - run: ./bin/unregistered verify\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{SsotArtifacts: []SsotArtifact{
		{ID: "registered-tool", Kind: "verifier", Path: "tools/registered/main.go"},
	}}
	errs := VerifyWorkflowToolCoverage(m, wfDir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "unregistered") || !strings.Contains(errs[0].Error(), "ci.yml") {
		t.Errorf("violation should cite tool and workflow, got %q", errs[0].Error())
	}
}

func TestVerifyWorkflowToolCoverageDeduplicatesAcrossSteps(t *testing.T) {
	// Same tool referenced 3 times in one workflow → still one violation.
	root := t.TempDir()
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"),
		[]byte("steps:\n  - run: ./bin/foo a\n  - run: ./bin/foo b\n  - run: ./bin/foo c\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{}
	errs := VerifyWorkflowToolCoverage(m, wfDir)
	if len(errs) != 1 {
		t.Errorf("expected 1 violation (deduplicated), got %d: %v", len(errs), errs)
	}
}

func TestVerifyWorkflowToolCoverageMissingDirIsOK(t *testing.T) {
	// If the workflows dir doesn't exist at all, no error — caller knows.
	m := &Map{}
	if errs := VerifyWorkflowToolCoverage(m, "/nonexistent/path"); len(errs) != 0 {
		t.Errorf("missing dir should not produce errors, got %v", errs)
	}
}

func TestVerifyWorkflowToolCoverageIgnoresNonBinReferences(t *testing.T) {
	// References that don't look like `./bin/<name>` are not flagged.
	root := t.TempDir()
	wfDir := filepath.Join(root, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "ci.yml"),
		[]byte("steps:\n  - run: go test ./internal/...\n  - run: bin/foo  # missing leading ./\n  - run: ../bin/foo  # not at repo root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &Map{}
	if errs := VerifyWorkflowToolCoverage(m, wfDir); len(errs) != 0 {
		t.Errorf("non-./bin/ refs should not be flagged, got %v", errs)
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
