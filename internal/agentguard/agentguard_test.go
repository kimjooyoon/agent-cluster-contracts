package agentguard

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pat, name string
		want      bool
	}{
		{"fixtures/positive/**", "fixtures/positive/a.json", true},
		{"fixtures/positive/**", "fixtures/positive/sub/a.json", true},
		{"fixtures/positive/**", "fixtures/negative/a.json", false},
		{"tools/**", "tools/decision/main.go", true},
		{"tools/**", "backend/main.go", false},
		{"**/*_test.go", "internal/agentguard/agentguard_test.go", true},
		{"**/*_test.go", "internal/agentguard/agentguard.go", false},
		{".github/workflows/**", ".github/workflows/contracts.yml", true},
		{"ssot-dependency-map.riido.json", "ssot-dependency-map.riido.json", true},
		{"ssot-dependency-map.riido.json", "ssot-dependency-map.riido.json.bak", false},
		{"**", "anything/anywhere.txt", true},
	}
	for _, c := range cases {
		got := matchGlob(c.pat, c.name)
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pat, c.name, got, c.want)
		}
	}
}

func TestPatternPrefix(t *testing.T) {
	cases := map[string]string{
		"fixtures/positive/**": "fixtures/positive",
		"tools/**":             "tools",
		"**/*_test.go":         "",
		"literal/file.json":    "literal/file.json",
		"a/*/b":                "a",
	}
	for in, want := range cases {
		if got := patternPrefix(in); got != want {
			t.Errorf("patternPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSegmentDiff(t *testing.T) {
	cases := []struct {
		pattern        string
		path           string
		wantMatched    int
		wantMissing    string
		desc           string
	}{
		{
			pattern: "fixtures/positive/**",
			path:    "fixtures/positive/x.json",
			// pattern prefix = ["fixtures","positive"]; matches both segments
			wantMatched: 2,
			wantMissing: "",
			desc:        "exact prefix match",
		},
		{
			pattern: "contracts/fixtures/positive/**",
			path:    "fixtures/positive/x.json",
			// pattern prefix = ["contracts","fixtures","positive"];
			// drop "contracts" → matches 2 segments
			wantMatched: 2,
			wantMissing: "contracts",
			desc:        "missing prefix detected",
		},
		{
			pattern: "tools/**",
			path:    "fixtures/positive/x.json",
			// no overlap
			wantMatched: 0,
			wantMissing: "",
			desc:        "no overlap",
		},
		{
			pattern: "a/b/c/d/**",
			path:    "c/d/x",
			// drop "a/b" → matches 2 segments
			wantMatched: 2,
			wantMissing: "a/b",
			desc:        "missing multi-segment prefix",
		},
	}
	for _, c := range cases {
		gotMatched, gotMissing := segmentDiff(c.pattern, strings.Split(c.path, "/"))
		if gotMatched != c.wantMatched || gotMissing != c.wantMissing {
			t.Errorf("%s: segmentDiff(%q, %q) = (%d, %q), want (%d, %q)",
				c.desc, c.pattern, c.path, gotMatched, gotMissing, c.wantMatched, c.wantMissing)
		}
	}
}

func TestCheckForbiddenTakesPrecedence(t *testing.T) {
	// A path that matches both forbidden and allowed should be reported once
	// as forbidden, not as allowed and not as not_allowed.
	r := &Role{
		ID:             "dumb-agent",
		AllowedPaths:   []string{"**"},
		ForbiddenPaths: []string{"tools/**"},
	}
	res := Check(r, []string{"tools/decision/main.go"})
	if res.OK {
		t.Fatal("expected violations, got ok")
	}
	if len(res.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %v", len(res.Violations), res.Violations)
	}
	v := res.Violations[0]
	if v.Kind != KindForbidden {
		t.Errorf("kind = %q, want %q", v.Kind, KindForbidden)
	}
	if v.MatchedPattern != "tools/**" {
		t.Errorf("matched_pattern = %q, want tools/**", v.MatchedPattern)
	}
	if !strings.Contains(v.Reason, "FORBIDDEN") {
		t.Errorf("reason missing FORBIDDEN marker: %q", v.Reason)
	}
}

func TestCheckNotAllowedSuggestsClosest(t *testing.T) {
	r := &Role{
		ID:           "dumb-agent",
		AllowedPaths: []string{"fixtures/positive/**", "fuzz/corpus/**"},
	}
	res := Check(r, []string{"fixtures/negative/x.json"})
	if len(res.Violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(res.Violations))
	}
	v := res.Violations[0]
	if v.Kind != KindNotAllowed {
		t.Errorf("kind = %q, want not_allowed", v.Kind)
	}
	if v.ClosestAllowed != "fixtures/positive/**" {
		t.Errorf("closest_allowed = %q, want fixtures/positive/**", v.ClosestAllowed)
	}
}

func TestCheckMissingPrefixHint(t *testing.T) {
	// This is the exact bug-class from the 41-violation incident.
	r := &Role{
		ID:           "dumb-agent",
		AllowedPaths: []string{"contracts/fixtures/positive/**", "contracts/fuzz/corpus/**"},
	}
	res := Check(r, []string{
		"fixtures/positive/a.json",
		"fixtures/positive/b.json",
		"fuzz/corpus/c.json",
	})
	if len(res.Violations) != 3 {
		t.Fatalf("expected 3 violations, got %d", len(res.Violations))
	}
	for _, v := range res.Violations {
		if v.MissingPrefix != "contracts" {
			t.Errorf("%s: missing_prefix = %q, want contracts", v.Path, v.MissingPrefix)
		}
	}
	// Global drift hint should fire when ≥2 violations share the same prefix.
	if len(res.Hints) == 0 || !strings.Contains(res.Hints[0], "prefix_drift") {
		t.Errorf("expected prefix_drift hint, got %v", res.Hints)
	}
	if !strings.Contains(res.Hints[0], `"contracts"`) {
		t.Errorf("drift hint missing the prefix string: %v", res.Hints[0])
	}
}

func TestCheckPrefixDriftQuietForSingleMiss(t *testing.T) {
	r := &Role{
		ID:           "dumb-agent",
		AllowedPaths: []string{"contracts/fixtures/positive/**"},
	}
	res := Check(r, []string{"fixtures/positive/a.json"}) // only one violation
	for _, h := range res.Hints {
		if strings.Contains(h, "prefix_drift") {
			t.Errorf("did not expect prefix_drift hint for a single match: %v", res.Hints)
		}
	}
}

func TestCheckMaxFilesIncludesSplitHint(t *testing.T) {
	r := &Role{
		ID:            "dumb-agent",
		AllowedPaths:  []string{"**"},
		MaxFilesPerPR: 2,
	}
	res := Check(r, []string{"a", "b", "c"})
	if len(res.Violations) == 0 {
		t.Fatal("expected at least one violation")
	}
	v := res.Violations[0]
	if v.Kind != KindMaxFiles {
		t.Errorf("first violation kind = %q, want max_files", v.Kind)
	}
	if !strings.Contains(v.Reason, "split into multiple PRs") {
		t.Errorf("max_files reason missing split hint: %q", v.Reason)
	}
}

func TestCheckResultJSONShapeIsStable(t *testing.T) {
	r := &Role{
		ID:             "dumb-agent",
		AllowedPaths:   []string{"fixtures/positive/**"},
		ForbiddenPaths: []string{"tools/**"},
	}
	res := Check(r, []string{
		"fixtures/positive/a.json", // allowed
		"tools/x.go",               // forbidden
		"other/y.json",             // not allowed
	})
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`"role":"dumb-agent"`,
		`"ok":false`,
		`"kind":"forbidden"`,
		`"kind":"not_allowed"`,
		`"matched_pattern":"tools/**"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON missing %q\n--- got ---\n%s", want, s)
		}
	}
}

func TestCheckHappyPath(t *testing.T) {
	r := &Role{
		ID:           "dumb-agent",
		AllowedPaths: []string{"fixtures/positive/**", "fuzz/corpus/**"},
	}
	res := Check(r, []string{
		"fixtures/positive/a.json",
		"fuzz/corpus/b.json",
	})
	if !res.OK {
		t.Fatalf("expected ok=true, got %+v", res)
	}
	if len(res.Violations) != 0 {
		t.Errorf("expected no violations, got %v", res.Violations)
	}
}

// ValidateRoles tests (decision 009).

func validRoles() *Roles {
	return &Roles{
		Version: "0.4.0",
		Owner:   "agent-cluster-contracts",
		Roles: []Role{
			{
				ID:             "designer",
				Label:          "Designer",
				Description:    "Primary author.",
				AllowedPaths:   []string{"**"},
				ForbiddenPaths: []string{},
			},
		},
	}
}

func TestValidateRolesHappyPath(t *testing.T) {
	if errs := ValidateRoles(validRoles()); len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateRolesRejectsBadVersion(t *testing.T) {
	r := validRoles()
	r.Version = "1"
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected version error")
	}
}

func TestValidateRolesRejectsBadOwner(t *testing.T) {
	r := validRoles()
	r.Owner = "someone-else"
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected owner error")
	}
}

func TestValidateRolesRejectsEmptyRoles(t *testing.T) {
	r := validRoles()
	r.Roles = nil
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected empty-roles error")
	}
}

func TestValidateRolesRejectsBadRoleID(t *testing.T) {
	r := validRoles()
	r.Roles[0].ID = "BadID"
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected role-id error")
	}
}

func TestValidateRolesRejectsDuplicateID(t *testing.T) {
	r := validRoles()
	r.Roles = append(r.Roles, Role{
		ID:             "designer",
		Label:          "dup",
		Description:    "x",
		AllowedPaths:   []string{},
		ForbiddenPaths: []string{},
	})
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected duplicate-id error")
	}
}

func TestValidateRolesRejectsMissingLabel(t *testing.T) {
	r := validRoles()
	r.Roles[0].Label = ""
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected label error")
	}
}

func TestValidateRolesRejectsNullAllowedPaths(t *testing.T) {
	r := validRoles()
	r.Roles[0].AllowedPaths = nil
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected allowed_paths-required error (nil vs empty array)")
	}
}

func TestValidateRolesRejectsEmptyStringPath(t *testing.T) {
	r := validRoles()
	r.Roles[0].AllowedPaths = []string{"valid/**", "  "}
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected empty-path error")
	}
}

func TestValidateRolesRejectsBadDecisionRef(t *testing.T) {
	r := validRoles()
	r.Roles[0].Decision = "not-a-decision"
	errs := ValidateRoles(r)
	if len(errs) == 0 {
		t.Error("expected decision-ref pattern error")
	}
}

func TestValidateRolesAcceptsActualSSOT(t *testing.T) {
	// Smoke: the actual contracts agent-roles.riido.json must validate
	// against ValidateRoles after decision 009 ships. We load via the
	// findroot mechanism using the test's working directory.
	//
	// This test runs from internal/agentguard so the contracts root is
	// two levels up.
	roles := &Roles{
		Version: "0.4.0",
		Owner:   "agent-cluster-contracts",
		Roles: []Role{
			{
				ID:             "designer",
				Label:          "Designer / human-driven implementer",
				Description:    "Primary author class.",
				Decision:       "001-initial-agreement",
				AllowedPaths:   []string{"**"},
				ForbiddenPaths: []string{},
			},
			{
				ID:             "dumb-agent",
				Label:          "Contract Fuzzer Agent / IR Mutation Scout",
				Description:    "Low-context closed-loop probe.",
				Decision:       "005-dumb-agent-probe-baseline-and-fixture-verifier",
				AllowedPaths:   []string{"fixtures/positive/**", "fixtures/negative/**"},
				ForbiddenPaths: []string{"tools/**", "decisions/**"},
				MaxFilesPerPR:  5,
				PRIsolation:    PRIsolation{CandidateOnly: true},
			},
		},
	}
	if errs := ValidateRoles(roles); len(errs) > 0 {
		t.Errorf("realistic roles failed validation: %v", errs)
	}
}
