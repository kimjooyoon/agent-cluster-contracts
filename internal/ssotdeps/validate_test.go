package ssotdeps

import "testing"

func validMap() *Map {
	return &Map{
		Version: "0.9.0",
		Owner:   "agent-cluster-contracts",
		SsotArtifacts: []SsotArtifact{
			{ID: "x", Kind: "decision", Path: "decisions/schema.riido.json", OwnedBy: "agent-cluster-contracts"},
		},
	}
}

func TestValidateMapHappyPath(t *testing.T) {
	if errs := ValidateMap(validMap()); len(errs) > 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateMapBadVersion(t *testing.T) {
	m := validMap()
	m.Version = "1"
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected version error")
	}
}

func TestValidateMapBadOwner(t *testing.T) {
	m := validMap()
	m.Owner = "elsewhere"
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected owner error")
	}
}

func TestValidateMapEmptyArtifacts(t *testing.T) {
	m := validMap()
	m.SsotArtifacts = nil
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected empty-artifacts error")
	}
}

func TestValidateMapBadArtifactID(t *testing.T) {
	m := validMap()
	m.SsotArtifacts[0].ID = "BadID"
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected id pattern error")
	}
}

func TestValidateMapDuplicateID(t *testing.T) {
	m := validMap()
	m.SsotArtifacts = append(m.SsotArtifacts, SsotArtifact{
		ID: "x", Kind: "decision", Path: "p", OwnedBy: "agent-cluster-contracts",
	})
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected duplicate-id error")
	}
}

func TestValidateMapBadKind(t *testing.T) {
	m := validMap()
	m.SsotArtifacts[0].Kind = "not-a-kind"
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected kind enum error")
	}
}

func TestValidateMapConsumptionLinkUnknownSSOT(t *testing.T) {
	m := validMap()
	m.ConsumptionLinks = []ConsumptionLink{{
		SSOT: "does-not-exist", ConsumerRepo: "agent-cluster-contracts", ConsumerPath: "p",
	}}
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected unknown-ssot error")
	}
}

func TestValidateMapCIGateAcceptsFreeFormVerifies(t *testing.T) {
	// `verifies` accepts either artifact IDs or free-form concern strings —
	// matching existing dep map usage where security.yml says
	// "raw-secret patterns in working tree" rather than an artifact ID.
	m := validMap()
	m.CIGates = []CIGate{{
		Repo: "agent-cluster-contracts", Workflow: ".github/workflows/security.yml",
		Verifies: []string{"raw-secret patterns in working tree"},
	}}
	if errs := ValidateMap(m); len(errs) != 0 {
		t.Errorf("expected free-form verifies to validate, got %v", errs)
	}
}

func TestValidateMapCIGateEmptyVerifyItemFails(t *testing.T) {
	m := validMap()
	m.CIGates = []CIGate{{
		Repo: "agent-cluster-contracts", Workflow: ".github/workflows/x.yml",
		Verifies: []string{""},
	}}
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected empty-verify-item error")
	}
}

func TestValidateMapCIGateEmptyVerifies(t *testing.T) {
	m := validMap()
	m.CIGates = []CIGate{{
		Repo: "agent-cluster-contracts", Workflow: ".github/workflows/x.yml",
		Verifies: []string{},
	}}
	if errs := ValidateMap(m); len(errs) == 0 {
		t.Error("expected empty-verifies error")
	}
}
