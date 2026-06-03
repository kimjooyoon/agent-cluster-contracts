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
		`{"fixture_type":"work-item","expected":"pass"}`)
	res, _ := VerifyFixtures(root)
	if res.OK {
		t.Errorf("expected failure on unsupported fixture_type (v1 supports decision only), got OK")
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
