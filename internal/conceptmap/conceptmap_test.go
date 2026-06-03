package conceptmap

import (
	"strings"
	"testing"
)

// validMap returns a minimal-but-valid Map used as the baseline; each
// test flips exactly one field to check rejection (mirrors the test
// idiom established for internal/decision in D036).
func validMap() *Map {
	return &Map{
		Version: "0.1.0",
		Owner:   "agent-cluster-contracts",
		Concepts: []Concept{
			{
				Name:        "Decision",
				Kind:        "artifact",
				Owner:       "agent-cluster-contracts",
				Description: "A recorded decision.",
				Examples:    []string{"001-initial-agreement"},
			},
		},
		Constraints: []Constraint{
			{
				ID:        "C-001",
				Rule:      "Some rule statement.",
				Rationale: "Some rationale prose.",
				Examples:  []string{"an example"},
			},
		},
	}
}

func TestValidateAcceptsMinimalMap(t *testing.T) {
	if errs := Validate(validMap()); len(errs) != 0 {
		t.Errorf("minimal map should validate, got %v", errs)
	}
}

func TestValidateRejectsBadVersion(t *testing.T) {
	// Note: the current check is just "must have exactly 2 dots" — so
	// values like "v1.2.3" or "abc.def.ghi" pass. This test documents
	// what's actually rejected. Tightening the check is a separate
	// decision (would change accepted inputs).
	cases := []string{"", "1", "1.0", "1.0.0.0"}
	for _, v := range cases {
		t.Run(v, func(t *testing.T) {
			m := validMap()
			m.Version = v
			errs := Validate(m)
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "version") {
					found = true
				}
			}
			if !found {
				t.Errorf("version=%q: expected version error, got %v", v, errs)
			}
		})
	}
}

func TestValidateRejectsNonPascalConceptName(t *testing.T) {
	// Note: the nameRe is ^[A-Z][A-Za-z0-9]*$ — accepts all-uppercase like
	// "DECISION" or "IRSchema" because the project has acronym-heavy names
	// (IRSchema, GraphQLQuery, SsotDependencyMap). This test covers what
	// the regex actually rejects: lowercase start, hyphens/punctuation,
	// digit start, empty.
	cases := []string{"decision", "Decision-2", "decisionTwo", "1Decision", "", "decision.x"}
	for _, n := range cases {
		t.Run(n, func(t *testing.T) {
			m := validMap()
			m.Concepts[0].Name = n
			errs := Validate(m)
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "PascalCase") {
					found = true
				}
			}
			if !found {
				t.Errorf("name=%q: expected PascalCase error, got %v", n, errs)
			}
		})
	}
}

func TestValidateRejectsInvalidConceptKind(t *testing.T) {
	m := validMap()
	m.Concepts[0].Kind = "not-a-kind"
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "kind") && strings.Contains(e.Error(), "not-a-kind") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected kind error, got %v", errs)
	}
}

func TestValidateRejectsUnknownOwner(t *testing.T) {
	m := validMap()
	m.Concepts[0].Owner = "agent-cluster-mystery"
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "owner") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected owner error, got %v", errs)
	}
}

func TestValidateRejectsMissingDescriptionOrExamples(t *testing.T) {
	m := validMap()
	m.Concepts[0].Description = ""
	m.Concepts[0].Examples = nil
	errs := Validate(m)
	descHit, exHit := false, false
	for _, e := range errs {
		if strings.Contains(e.Error(), "description") {
			descHit = true
		}
		if strings.Contains(e.Error(), "examples") {
			exHit = true
		}
	}
	if !descHit || !exHit {
		t.Errorf("expected both description and examples errors, got %v (desc=%v ex=%v)", errs, descHit, exHit)
	}
}

func TestValidateRejectsDuplicateConceptName(t *testing.T) {
	m := validMap()
	m.Concepts = append(m.Concepts, m.Concepts[0]) // copy
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate-name error, got %v", errs)
	}
}

func TestValidateRejectsRelationshipReferencingUnknownConcept(t *testing.T) {
	m := validMap()
	m.Relationships = []Relationship{
		{From: "Decision", To: "Ghost", Kind: "owns", Example: "x"},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "Ghost") && strings.Contains(e.Error(), "not a declared concept") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown-concept relationship error, got %v", errs)
	}
}

func TestValidateRejectsInvalidRelationshipKind(t *testing.T) {
	m := validMap()
	m.Relationships = []Relationship{
		{From: "Decision", To: "Decision", Kind: "loves", Example: "x"},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "loves") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unknown-relationship-kind error, got %v", errs)
	}
}

func TestValidateRejectsInvalidConstraintID(t *testing.T) {
	cases := []string{"c-001", "C-1", "C-0001", "X-001", ""}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			m := validMap()
			m.Constraints[0].ID = id
			errs := Validate(m)
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "C-[0-9]{3}") {
					found = true
				}
			}
			if !found {
				t.Errorf("id=%q: expected id-pattern error, got %v", id, errs)
			}
		})
	}
}

func TestValidateRejectsDuplicateConstraintID(t *testing.T) {
	m := validMap()
	m.Constraints = append(m.Constraints, m.Constraints[0])
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate id") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected duplicate-id error, got %v", errs)
	}
}

func TestValidateRejectsInvalidGuardStatus(t *testing.T) {
	m := validMap()
	m.Constraints[0].GuardCandidate = &GuardCand{Tool: "x", Status: "halfway"}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "halfway") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid-guard-status error, got %v", errs)
	}
}

func TestValidateAcceptsKnownGuardStatuses(t *testing.T) {
	for _, st := range []string{"pending", "implemented", "deprecated"} {
		t.Run(st, func(t *testing.T) {
			m := validMap()
			m.Constraints[0].GuardCandidate = &GuardCand{Tool: "x", Status: st}
			if errs := Validate(m); len(errs) != 0 {
				t.Errorf("status=%q should validate, got %v", st, errs)
			}
		})
	}
}

func TestQueryFindsConceptCaseInsensitive(t *testing.T) {
	m := validMap()
	for _, q := range []string{"Decision", "decision", "DECISION", "dEcIsIoN"} {
		t.Run(q, func(t *testing.T) {
			c, _, _ := m.Query(q)
			if c == nil {
				t.Errorf("query=%q: expected Decision concept, got nil", q)
			}
		})
	}
}

func TestQueryReturnsNilForUnknownConcept(t *testing.T) {
	m := validMap()
	c, rels, cons := m.Query("Nonexistent")
	if c != nil {
		t.Errorf("expected nil concept, got %+v", c)
	}
	if len(rels) != 0 || len(cons) != 0 {
		t.Errorf("expected no rels/cons, got rels=%v cons=%v", rels, cons)
	}
}

func TestQueryReturnsRelationshipsThatMentionName(t *testing.T) {
	m := validMap()
	m.Concepts = append(m.Concepts, Concept{
		Name: "Receiver", Kind: "artifact", Owner: "agent-cluster-contracts",
		Description: "x", Examples: []string{"x"},
	})
	m.Relationships = []Relationship{
		{From: "Decision", To: "Receiver", Kind: "owns", Example: "x"},
		{From: "Receiver", To: "Decision", Kind: "references", Example: "x"},
	}
	_, rels, _ := m.Query("Decision")
	if len(rels) != 2 {
		t.Errorf("expected 2 rels mentioning Decision, got %d", len(rels))
	}
}

func TestQueryReturnsConstraintsMentioningName(t *testing.T) {
	m := validMap()
	m.Constraints = []Constraint{
		{ID: "C-001", Rule: "Decision records must be valid.", Rationale: "x", Examples: []string{"x"}},
		{ID: "C-002", Rule: "Unrelated to anything.", Rationale: "Decision-driven rationale.", Examples: []string{"x"}},
		{ID: "C-003", Rule: "Nothing mentions us here.", Rationale: "Truly unrelated prose.", Examples: []string{"x"}},
	}
	_, _, cons := m.Query("Decision")
	if len(cons) != 2 {
		t.Errorf("expected 2 constraints mentioning Decision (in rule or rationale), got %d", len(cons))
	}
}
