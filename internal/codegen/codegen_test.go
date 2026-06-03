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
