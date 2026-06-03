package codegen

import "testing"

func TestPascal(t *testing.T) {
	cases := map[string]string{
		"work-item":         "WorkItem",
		"work-item-created": "WorkItemCreated",
		"id":                "Id",
		"":                  "",
		"single":            "Single",
	}
	for in, want := range cases {
		if got := Pascal(in); got != want {
			t.Errorf("Pascal(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCamel(t *testing.T) {
	cases := map[string]string{
		"work-item":         "workItem",
		"work-item-created": "workItemCreated",
		"id":                "id",
		"":                  "",
	}
	for in, want := range cases {
		if got := Camel(in); got != want {
			t.Errorf("Camel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSnake(t *testing.T) {
	cases := map[string]string{
		"work-item":         "work_item",
		"work-item-created": "work_item_created",
		"id":                "id",
	}
	for in, want := range cases {
		if got := Snake(in); got != want {
			t.Errorf("Snake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIRDocQueryShape(t *testing.T) {
	// Decision 006: kind=query IR documents carry WireName + Returns.
	// Test data uses a non-SSOT wire-name so wirelint (D008) does not flag
	// this file as a C-006 violation — codegen tests must not couple to
	// any real query name.
	const testWire = "testWireName"
	d := &IRDoc{
		Kind:     "query",
		Name:     "test-fixture",
		WireName: testWire,
		Returns:  &QueryReturns{Shape: "list", Type: "work-item"},
	}
	if d.WireName != testWire {
		t.Errorf("WireName = %q", d.WireName)
	}
	if d.Returns == nil || d.Returns.Shape != "list" || d.Returns.Type != "work-item" {
		t.Errorf("Returns wrong: %+v", d.Returns)
	}
	if len(d.Slots) != 0 {
		t.Errorf("query doc must not have slots, got %d", len(d.Slots))
	}
}
