// Package codegen holds the shared IR loader and naming helpers used by the
// gen-go-client and gen-dart-client tools. The IR shape is the same; only the
// emission target differs.
package codegen

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

// IRDoc mirrors the emitter output. Kind is "aggregate", "event", or "query".
// Fields are populated according to Kind; consumers must branch on Kind.
type IRDoc struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Source Source `json:"source"`

	// Slots applies to Kind=aggregate and Kind=event.
	Slots []Slot `json:"slots,omitempty"`

	// WireName + Returns apply to Kind=query (decision 006).
	WireName string         `json:"wire_name,omitempty"`
	Returns  *QueryReturns  `json:"returns,omitempty"`

	// Path the file was loaded from, relative to the contracts repo root.
	// Populated by LoadAll, not deserialized from JSON.
	Path string `json:"-"`
}

// QueryReturns describes a Kind=query result shape.
type QueryReturns struct {
	// Shape is "list" or "one".
	Shape string `json:"shape"`
	// Type is the kebab-case aggregate name (e.g. "work-item").
	Type string `json:"type"`
}

type Slot struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Required bool   `json:"required"`
}

type Source struct {
	DSLFile string `json:"dsl_file"`
	SHA256  string `json:"sha256"`
}

// LoadAll walks irDir (recursively), reads every *.ir.json into an IRDoc, and
// returns the slice sorted by Name so generators produce deterministic output
// across operating systems.
func LoadAll(irDir string) ([]*IRDoc, error) {
	var out []*IRDoc
	err := filepath.WalkDir(irDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".ir.json") {
			return nil
		}
		doc := &IRDoc{Path: path}
		if err := jsonutil.ReadFile(path, doc); err != nil {
			return err
		}
		out = append(out, doc)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Pascal converts kebab-case ("work-item-id") to PascalCase ("WorkItemId").
func Pascal(kebab string) string {
	var b strings.Builder
	upNext := true
	for _, r := range kebab {
		if r == '-' || r == '_' {
			upNext = true
			continue
		}
		if upNext {
			b.WriteRune(unicode.ToUpper(r))
			upNext = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Camel converts kebab-case ("work-item-id") to camelCase ("workItemId").
func Camel(kebab string) string {
	p := Pascal(kebab)
	if p == "" {
		return p
	}
	runes := []rune(p)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// Snake converts kebab-case ("work-item-id") to snake_case ("work_item_id").
// Used for JSON wire field names — chosen because Go's encoding/json convention
// is snake_case, and the wire format is what consumers actually see.
func Snake(kebab string) string {
	return strings.ReplaceAll(kebab, "-", "_")
}

// FileBase returns a snake-case base filename from a kebab IR name.
// "work-item-created" → "work_item_created"
func FileBase(kebab string) string {
	return Snake(kebab)
}
