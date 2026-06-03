package probe

import (
	"os"
	"path/filepath"
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
		`{"fixture_type":"decision","expected":"pass"}`)

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
		`{"fixture_type":"decision","expected":"fail","expected_error_contains":"title"}`)

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
		`{"fixture_type":"decision","expected":"fail","expected_error_contains":"this-string-not-present"}`)
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
		`{"fixture_type":"decision","expected":"pass"}`)
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
		`{"fixture_type":"unknown-type","expected":"pass"}`)
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
		`{"fixture_type":"ir-aggregate","expected":"pass"}`)
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
		`{"fixture_type":"ir-aggregate","expected":"fail","expected_error_contains":"at least one slot"}`)
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
		`{"fixture_type":"ir-aggregate","expected":"pass"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when meta promises ir-aggregate but doc kind is query, got OK")
	}
}

func TestFixturesQueryPositivePasses(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "fixtures/positive/query/list.json"), validIRQueryJSON())
	write(t, filepath.Join(root, "fixtures/positive/query/list.meta.json"),
		`{"fixture_type":"query","expected":"pass"}`)
	res, _ := VerifyFixtures(root)
	if !res.OK {
		t.Errorf("expected OK, got %+v", res.Checks)
	}
}

func TestFixturesIRAggregateRejectsExtraQueryFields(t *testing.T) {
	root := t.TempDir()
	// aggregate doc carrying a wire_name → schema rule says forbidden for non-query
	write(t, filepath.Join(root, "fixtures/negative/x/bad.json"),
		`{"kind":"aggregate","name":"x","slots":[{"name":"id","type":"string","required":true}],"wire_name":"oops","source":{"dsl_file":"dsl/x.lisp","sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}`)
	write(t, filepath.Join(root, "fixtures/negative/x/bad.meta.json"),
		`{"fixture_type":"ir-aggregate","expected":"fail","expected_error_contains":"must not declare wire_name"}`)
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
		`{"fixture_type":"decision","expected":"pass"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure when negative/ contains expected=pass, got OK")
	}
}
