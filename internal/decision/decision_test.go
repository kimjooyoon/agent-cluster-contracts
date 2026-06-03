package decision

import (
	"strings"
	"testing"
)

// validRecord returns a minimal record that passes Validate. Helper for
// tests that flip exactly one field to check rejection.
func validRecord() *Record {
	return &Record{
		ID:    "999-fixture-validate-test",
		Title: "test",
		Owner: "test",
		Status: "accepted",
		Scope: Scope{
			BoundedContexts: []string{"platform"},
			Areas:           []string{"governance"},
		},
		Source: "top_down",
		Evidence: []Evidence{
			{Kind: "log", Ref: "test"},
		},
		AffectedRepos: []string{"agent-cluster-contracts"},
		SsotOwner:     "agent-cluster-contracts",
		Examples:      []string{"x"},
		CreatedAt:     "2026-06-03",
	}
}

// D036 — bounded_contexts enum: must reject unknown BCs.

func TestValidateAcceptsKnownBoundedContexts(t *testing.T) {
	cases := []string{"platform", "work-item"}
	for _, bc := range cases {
		r := validRecord()
		r.Scope.BoundedContexts = []string{bc}
		if errs := Validate(r); len(errs) != 0 {
			t.Errorf("BC=%q should be accepted, got %v", bc, errs)
		}
	}
}

func TestValidateRejectsUnknownBoundedContexts(t *testing.T) {
	cases := []string{"NotARealBC", "platfrm", "", "Platform", "work_item"}
	for _, bc := range cases {
		t.Run(bc, func(t *testing.T) {
			r := validRecord()
			r.Scope.BoundedContexts = []string{bc}
			errs := Validate(r)
			if len(errs) == 0 {
				t.Errorf("BC=%q should be rejected, got OK", bc)
				return
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "bounded_contexts") && strings.Contains(e.Error(), bc) {
					found = true
				}
			}
			if !found {
				t.Errorf("BC=%q: expected 'bounded_contexts' error mentioning the bad value, got %v", bc, errs)
			}
		})
	}
}

func TestValidateRejectsMixedKnownAndUnknownBoundedContexts(t *testing.T) {
	// One known + one unknown → exactly one violation for the unknown one.
	r := validRecord()
	r.Scope.BoundedContexts = []string{"platform", "NotARealBC"}
	errs := Validate(r)
	if len(errs) != 1 {
		t.Fatalf("expected 1 BC violation, got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "NotARealBC") {
		t.Errorf("expected violation to cite NotARealBC, got %q", errs[0].Error())
	}
}
