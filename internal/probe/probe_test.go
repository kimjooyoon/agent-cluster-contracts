package probe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: write a JSON-ish file
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// validDecisionJSON returns a minimal-but-valid decision record JSON.
func validDecisionJSON() string {
	return `{
  "id": "999-fixture-positive-min",
  "title": "fixture",
  "owner": "test",
  "status": "accepted",
  "source": "top_down",
  "scope": {"bounded_contexts": [], "areas": ["governance"]},
  "evidence": [{"kind": "file", "ref": "x"}],
  "affected_repos": ["agent-cluster-contracts"],
  "ssot_owner": "agent-cluster-contracts",
  "generated_artifacts": [],
  "guards": [],
  "examples": ["x"],
  "counterexamples": [],
  "created_at": "2026-06-03"
}`
}

func TestFixturesEmptyTreeIsOK(t *testing.T) {
	root := t.TempDir()
	res, err := VerifyFixtures(root)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("expected OK on empty tree, got %+v", res)
	}
}

func TestFixturesPositiveDecisionPasses(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/decision/min.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/min.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"minimal valid decision"}`)

	res, err := VerifyFixtures(root)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("expected OK, got %+v", res)
	}
	if len(res.Checks) != 1 || !res.Checks[0].OK {
		t.Errorf("expected 1 OK check, got %+v", res.Checks)
	}
}

func TestFixturesNegativeDecisionFailsAsExpected(t *testing.T) {
	root := t.TempDir()
	// invalid: missing required fields (no title)
	write(t, filepath.Join(root, "fixtures/negative/decision/no-title.json"),
		`{"id":"999-x","owner":"t","status":"accepted","source":"top_down","scope":{"bounded_contexts":[],"areas":[]},"evidence":[{"kind":"file","ref":"x"}],"affected_repos":["agent-cluster-contracts"],"ssot_owner":"agent-cluster-contracts","generated_artifacts":[],"guards":[],"examples":["x"],"counterexamples":[],"created_at":"2026-06-03"}`)
	write(t, filepath.Join(root, "fixtures/negative/decision/no-title.meta.json"),
		`{"fixture_type":"decision","expected":"fail","expected_error_contains":"title","purpose":"decision missing title field"}`)

	res, err := VerifyFixtures(root)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("expected OK (negative fixture failed as expected), got %+v", res.Checks)
	}
}

func TestFixturesNegativeMissingExpectedSubstringFails(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/negative/decision/bad.json"),
		`{"id":"999-x","owner":"t","status":"accepted","source":"top_down","scope":{"bounded_contexts":[],"areas":[]},"evidence":[{"kind":"file","ref":"x"}],"affected_repos":["agent-cluster-contracts"],"ssot_owner":"agent-cluster-contracts","generated_artifacts":[],"guards":[],"examples":["x"],"counterexamples":[],"created_at":"2026-06-03"}`)
	write(t, filepath.Join(root, "fixtures/negative/decision/bad.meta.json"),
		`{"fixture_type":"decision","expected":"fail","expected_error_contains":"this-string-not-present","purpose":"test bad expected substring"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when expected_error_contains is missing from actual error, got OK")
	}
}

func TestFixturesPositiveThatAccidentallyFailsFlagsGap(t *testing.T) {
	root := t.TempDir()
	// missing required fields → would NOT validate; marked as positive → expected
	// pass; the verifier must flag this so the dumb-agent doesn't silently
	// commit broken positive fixtures.
	write(t, filepath.Join(root, "fixtures/positive/decision/broken.json"), `{"id":"missing","status":"accepted"}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/broken.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"intentionally broken positive"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure on broken positive fixture, got OK")
	}
}

func TestFixturesMetaWithMissingFieldsRejected(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/decision/x.json"), `{}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/x.meta.json"), `{}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when meta is missing fixture_type/expected, got OK")
	}
}

func TestFixturesUnsupportedTypeRejected(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/work-item/x.json"), `{}`)
	write(t, filepath.Join(root, "fixtures/positive/work-item/x.meta.json"),
		`{"fixture_type":"unknown-type","expected":"pass","purpose":"unsupported fixture_type test"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure on unsupported fixture_type, got OK")
	}
}

// validIRAggregateJSON returns a minimal-but-valid IR aggregate document.
func validIRAggregateJSON() string {
	return `{
  "kind": "aggregate",
  "name": "demo-thing",
  "slots": [
    {"name": "id", "type": "string", "required": true}
  ],
  "source": {
    "dsl_file": "dsl/domain/demo-thing.lisp",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}`
}

func validIRQueryJSON() string {
	return `{
  "kind": "query",
  "name": "list-things",
  "wire_name": "things",
  "returns": {"shape": "list", "type": "demo-thing"},
  "source": {
    "dsl_file": "dsl/domain/list-things.lisp",
    "sha256": "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
  }
}`
}

func TestFixturesIRAggregatePositivePasses(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/work-item/min.json"), validIRAggregateJSON())
	write(t, filepath.Join(root, "fixtures/positive/work-item/min.meta.json"),
		`{"fixture_type":"ir-aggregate","expected":"pass","purpose":"minimal valid ir aggregate"}`)
	res, err := VerifyFixtures(root)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Errorf("expected OK, got %+v", res.Checks)
	}
}

func TestFixturesIRAggregateNegativeMissingSlotsFails(t *testing.T) {
	root := t.TempDir()
	// Aggregate with empty slots array — schema requires ≥1 slot.
	write(t, filepath.Join(root, "fixtures/negative/work-item/no-slots.json"),
		`{"kind":"aggregate","name":"x","slots":[],"source":{"dsl_file":"dsl/x.lisp","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}`)
	write(t, filepath.Join(root, "fixtures/negative/work-item/no-slots.meta.json"),
		`{"fixture_type":"ir-aggregate","expected":"fail","expected_error_contains":"at least one slot","purpose":"ir aggregate with empty slots"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected OK (negative IR aggregate failed as expected): %+v", res.Checks)
	}
}

func TestFixturesIRAggregateKindMismatchFails(t *testing.T) {
	root := t.TempDir()
	// Document is a query but meta promises ir-aggregate.
	write(t, filepath.Join(root, "fixtures/positive/x/bad.json"), validIRQueryJSON())
	write(t, filepath.Join(root, "fixtures/positive/x/bad.meta.json"),
		`{"fixture_type":"ir-aggregate","expected":"pass","purpose":"meta lies about kind"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when meta promises ir-aggregate but doc kind is query, got OK")
	}
}

func TestFixturesQueryPositivePasses(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/query/list.json"), validIRQueryJSON())
	write(t, filepath.Join(root, "fixtures/positive/query/list.meta.json"),
		`{"fixture_type":"query","expected":"pass","purpose":"minimal valid query"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected OK, got %+v", res.Checks)
	}
}

// Decision 015 — purpose required + dedup.

func TestFixturesMissingPurposeRejected(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/decision/x.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/x.meta.json"),
		`{"fixture_type":"decision","expected":"pass"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when meta.purpose missing, got OK")
	}
	hasReason := false
	for _, c := range res.Checks {
		if strings.Contains(c.Reason, "purpose required") {
			hasReason = true
		}
	}
	if !hasReason {
		t.Errorf("expected 'purpose required' reason, got %+v", res.Checks)
	}
}

func TestFixturesEmptyPurposeRejected(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/decision/x.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/x.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"   "}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when meta.purpose is whitespace-only, got OK")
	}
}

func TestFixturesDuplicatePurposeRejected(t *testing.T) {
	root := t.TempDir()
	// Two positive decision fixtures with the SAME purpose but different
	// id/title content — the cycle-fixture noise pattern. Decision 015 says
	// the second one is a violation regardless of content differences.
	write(t, filepath.Join(root, "fixtures/positive/decision/a.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/a.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"minimal valid decision"}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/b.json"),
		`{"id":"999-other","title":"different title","owner":"t","status":"accepted","source":"top_down","scope":{"bounded_contexts":[],"areas":["governance"]},"evidence":[{"kind":"file","ref":"x"}],"affected_repos":["agent-cluster-contracts"],"ssot_owner":"agent-cluster-contracts","generated_artifacts":[],"guards":[],"examples":["x"],"counterexamples":[],"created_at":"2026-06-03"}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/b.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"minimal valid decision"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when two positive decisions share purpose, got OK")
	}
	dup := false
	for _, c := range res.Checks {
		if strings.Contains(c.Reason, "duplicate purpose") {
			dup = true
		}
	}
	if !dup {
		t.Errorf("expected 'duplicate purpose' reason, got %+v", res.Checks)
	}
}

func TestFixturesSamePurposeAcrossCategoriesAllowed(t *testing.T) {
	root := t.TempDir()
	// Positive and negative fixtures can share a purpose — they're in
	// different categories so they test different surfaces.
	write(t, filepath.Join(root, "fixtures/positive/decision/a.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/a.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"required field coverage"}`)
	write(t, filepath.Join(root, "fixtures/negative/decision/b.json"),
		`{"id":"999-no-title","owner":"t","status":"accepted","source":"top_down","scope":{"bounded_contexts":[],"areas":[]},"evidence":[{"kind":"file","ref":"x"}],"affected_repos":["agent-cluster-contracts"],"ssot_owner":"agent-cluster-contracts","generated_artifacts":[],"guards":[],"examples":["x"],"counterexamples":[],"created_at":"2026-06-03"}`)
	write(t, filepath.Join(root, "fixtures/negative/decision/b.meta.json"),
		`{"fixture_type":"decision","expected":"fail","expected_error_contains":"title","purpose":"required field coverage"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected OK (different categories), got %+v", res.Checks)
	}
}

// Decision 017 — purpose normalization (catches timestamp-suffix gaming).

func TestNormalizePurposeStripsParenthesizedTimestamp(t *testing.T) {
	a := NormalizePurpose("Ensure a unique accepted decision fixture validates successfully (1780485421)")
	b := NormalizePurpose("Ensure a unique accepted decision fixture validates successfully (1780485142)")
	if a != b {
		t.Errorf("expected normalized forms to match (stripped timestamps), got %q vs %q", a, b)
	}
}

func TestNormalizePurposeStripsCycleTokens(t *testing.T) {
	cases := []string{
		"Positive fixture cycle 3",
		"Positive fixture cycle 14",
		"Positive fixture cycle-30",
		"Positive fixture CYCLE  99",
	}
	first := NormalizePurpose(cases[0])
	for _, c := range cases[1:] {
		if NormalizePurpose(c) != first {
			t.Errorf("expected %q to normalize same as %q (cycle stripped), got %q vs %q", c, cases[0], NormalizePurpose(c), first)
		}
	}
}

func TestNormalizePurposeStripsBareLongNumbers(t *testing.T) {
	a := NormalizePurpose("fixture-1780485421 verifies decision")
	b := NormalizePurpose("fixture-1780485142 verifies decision")
	if a != b {
		t.Errorf("expected bare-timestamp stripping to make these equal, got %q vs %q", a, b)
	}
}

func TestNormalizePurposePreservesMeaningfulDifferences(t *testing.T) {
	cases := map[string]string{
		"decision invalid missing title": "decision invalid missing title",
		"decision invalid missing owner": "decision invalid missing owner",
		"decision invalid missing scope": "decision invalid missing scope",
	}
	seen := map[string]string{}
	for in, _ := range cases {
		norm := NormalizePurpose(in)
		if prev, dup := seen[norm]; dup {
			t.Errorf("expected %q and %q to remain distinct after normalization, both → %q", prev, in, norm)
		}
		seen[norm] = in
	}
}

func TestFixturesRejectsTimestampSuffixedDuplicate(t *testing.T) {
	// The actual D017 enforcement case: dumb-agent cycle-style fixtures
	// with timestamp-suffixed "unique" purposes should be rejected as
	// duplicates because the normalized form matches.
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/decision/cycle-a.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/cycle-a.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"Ensure a unique accepted decision fixture validates successfully (1780485421)"}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/cycle-b.json"),
		`{"id":"999-cycle-b","title":"different title","owner":"t","status":"accepted","source":"top_down","scope":{"bounded_contexts":[],"areas":["governance"]},"evidence":[{"kind":"file","ref":"x"}],"affected_repos":["agent-cluster-contracts"],"ssot_owner":"agent-cluster-contracts","generated_artifacts":[],"guards":[],"examples":["x"],"counterexamples":[],"created_at":"2026-06-03"}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/cycle-b.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"Ensure a unique accepted decision fixture validates successfully (1780485142)"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected D017 to flag timestamp-suffixed duplicate, got OK")
	}
	dup := false
	for _, c := range res.Checks {
		if strings.Contains(c.Reason, "duplicate purpose") {
			dup = true
		}
	}
	if !dup {
		t.Errorf("expected 'duplicate purpose' reason in some check, got %+v", res.Checks)
	}
}

// Decision 018 — purpose banlist (persistent rejection of known templates).

func TestFixturesBanlistRejectsFirstOccurrence(t *testing.T) {
	// The exact gap D017 left: a SINGLE fixture (no other to dedup against)
	// using a banned template still gets rejected.
	root := t.TempDir()
	write(t, filepath.Join(root, "purpose-banlist.riido.json"),
		`{"version":"0.1.0","owner":"agent-cluster-contracts","banned":[
		  {"normalized":"ensure a unique accepted decision fixture validates successfully",
		   "seeded_by_decision":"018-fixture-purpose-banlist",
		   "reason":"test"}
		]}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/lone.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/lone.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"Ensure a unique accepted decision fixture validates successfully (9999999999)"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected banlist to reject first-occurrence banned template, got OK")
	}
	banned := false
	for _, c := range res.Checks {
		if strings.Contains(c.Reason, "banned purpose template") {
			banned = true
		}
	}
	if !banned {
		t.Errorf("expected 'banned purpose template' reason, got %+v", res.Checks)
	}
}

func TestFixturesBanlistDoesNotAffectLegitimatePurposes(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "purpose-banlist.riido.json"),
		`{"version":"0.1.0","owner":"agent-cluster-contracts","banned":[
		  {"normalized":"ensure a unique accepted decision fixture validates successfully",
		   "seeded_by_decision":"018-fixture-purpose-banlist",
		   "reason":"test"}
		]}`)
	write(t, filepath.Join(root, "fixtures/positive/decision/good.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/good.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"decision with all required fields present"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected legitimate purpose to pass, got %+v", res.Checks)
	}
}

func TestFixturesBanlistMissingFileIsOK(t *testing.T) {
	// No banlist file → no entries → nothing banned.
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/decision/lone.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/positive/decision/lone.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"some unique purpose"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected OK when banlist file is absent, got %+v", res.Checks)
	}
}

func TestFixturesIRAggregateRejectsExtraQueryFields(t *testing.T) {
	root := t.TempDir()
	// aggregate doc carrying a wire_name → schema rule says forbidden for non-query
	write(t, filepath.Join(root, "fixtures/negative/x/bad.json"),
		`{"kind":"aggregate","name":"x","slots":[{"name":"id","type":"string","required":true}],"wire_name":"oops","source":{"dsl_file":"dsl/x.lisp","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}`)
	write(t, filepath.Join(root, "fixtures/negative/x/bad.meta.json"),
		`{"fixture_type":"ir-aggregate","expected":"fail","expected_error_contains":"must not declare wire_name","purpose":"aggregate with stray wire_name"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected OK (negative caught extra wire_name): %+v", res.Checks)
	}
}

func TestFixturesCategoryDirectoryEnforcesExpected(t *testing.T) {
	root := t.TempDir()
	// Drop a 'expected: pass' under negative/ — that's a meta/category mismatch
	write(t, filepath.Join(root, "fixtures/negative/decision/x.json"), validDecisionJSON())
	write(t, filepath.Join(root, "fixtures/negative/decision/x.meta.json"),
		`{"fixture_type":"decision","expected":"pass","purpose":"category vs expected mismatch"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when negative/ contains expected=pass, got OK")
	}
}
