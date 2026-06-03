package agentguard

import "testing"

func TestGlobMatch(t *testing.T) {
	cases := []struct {
		pat, name string
		want      bool
	}{
		{"contracts/fixtures/positive/**", "contracts/fixtures/positive/a.json", true},
		{"contracts/fixtures/positive/**", "contracts/fixtures/positive/sub/a.json", true},
		{"contracts/fixtures/positive/**", "contracts/fixtures/negative/a.json", false},
		{"contracts/tools/**", "contracts/tools/decision/main.go", true},
		{"contracts/tools/**", "backend/main.go", false},
		{"**/*_test.go", "contracts/internal/agentguard/agentguard_test.go", true},
		{"**/*_test.go", "contracts/internal/agentguard/agentguard.go", false},
		{".github/workflows/**", ".github/workflows/contracts.yml", true},
		{"contracts/initial-agreement.md", "contracts/initial-agreement.md", true},
		{"contracts/initial-agreement.md", "contracts/initial-agreement.md.bak", false},
		{"**", "anything/anywhere.txt", true},
	}
	for _, c := range cases {
		got := matchGlob(c.pat, c.name)
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pat, c.name, got, c.want)
		}
	}
}

func TestCheckDumbAgentForbidsTools(t *testing.T) {
	r := &Role{
		ID:             "dumb-agent",
		AllowedPaths:   []string{"contracts/fixtures/positive/**", "contracts/fuzz/corpus/**"},
		ForbiddenPaths: []string{"contracts/tools/**", "contracts/decisions/**", ".github/workflows/**"},
	}
	vs := Check(r, []string{
		"contracts/fixtures/positive/wi-001.json",
		"contracts/tools/decision/main.go",
	})
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d (%v)", len(vs), vs)
	}
	if vs[0].Path != "contracts/tools/decision/main.go" {
		t.Errorf("wrong path in violation: %v", vs[0])
	}
}

func TestCheckDumbAgentAllowsCandidateOnly(t *testing.T) {
	r := &Role{
		ID:           "dumb-agent",
		AllowedPaths: []string{"contracts/fixtures/positive/**", "contracts/fuzz/corpus/**"},
	}
	vs := Check(r, []string{
		"contracts/fixtures/positive/wi-001.json",
		"contracts/fuzz/corpus/abc.json",
	})
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations, got %d (%v)", len(vs), vs)
	}
}

func TestCheckDumbAgentRejectsUnmatched(t *testing.T) {
	r := &Role{
		ID:           "dumb-agent",
		AllowedPaths: []string{"contracts/fixtures/positive/**"},
	}
	vs := Check(r, []string{"contracts/random/file.txt"})
	if len(vs) != 1 {
		t.Fatalf("expected 1 violation, got %d (%v)", len(vs), vs)
	}
}

func TestCheckMaxFiles(t *testing.T) {
	r := &Role{
		ID:            "dumb-agent",
		AllowedPaths:  []string{"**"},
		MaxFilesPerPR: 2,
	}
	vs := Check(r, []string{"a", "b", "c"})
	if len(vs) == 0 {
		t.Fatal("expected at least one violation for exceeding max_files_per_pr")
	}
}
