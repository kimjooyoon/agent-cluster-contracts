// Package conceptmap loads and validates contracts/concept-map/concept-map.riido.json.
package conceptmap

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

type Map struct {
	Version       string         `json:"version"`
	Owner         string         `json:"owner"`
	Concepts      []Concept      `json:"concepts"`
	Relationships []Relationship `json:"relationships"`
	Constraints   []Constraint   `json:"constraints"`
}

type Concept struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Owner       string   `json:"owner"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	Decision    *string  `json:"decision,omitempty"`
}

type Relationship struct {
	From     string  `json:"from"`
	Kind     string  `json:"kind"`
	To       string  `json:"to"`
	Example  string  `json:"example"`
	Decision *string `json:"decision,omitempty"`
}

type Constraint struct {
	ID              string      `json:"id"`
	Rule            string      `json:"rule"`
	Rationale       string      `json:"rationale"`
	Examples        []string    `json:"examples"`
	Counterexamples []string    `json:"counterexamples,omitempty"`
	GuardCandidate  *GuardCand  `json:"guard_candidate,omitempty"`
	Decision        *string     `json:"decision,omitempty"`
}

type GuardCand struct {
	Tool   string `json:"tool"`
	Status string `json:"status"`
}

var (
	nameRe    = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	cidRe     = regexp.MustCompile(`^C-[0-9]{3}$`)
	validKind = map[string]bool{
		"aggregate": true, "entity": true, "value-object": true, "event": true,
		"policy": true, "view": true, "process": true, "actor": true,
		"boundary": true, "artifact": true,
	}
	validRel = map[string]bool{
		"emits": true, "consumes": true, "owns": true, "references": true,
		"supersedes": true, "implements": true, "displays": true, "constrains": true,
	}
	validRepo = map[string]bool{
		"agent-cluster-contracts": true,
		"agent-cluster-backend":   true,
		"agent-cluster-frontend":  true,
	}
	validGuardStatus = map[string]bool{"pending": true, "implemented": true, "deprecated": true}
)

// Load reads the concept map at root/concept-map/concept-map.riido.json.
func Load(root string) (*Map, error) {
	m := &Map{}
	if err := jsonutil.ReadFile(filepath.Join(root, "concept-map", "concept-map.riido.json"), m); err != nil {
		return nil, err
	}
	return m, nil
}

// Validate enforces the schema and the structural rules (relationship endpoints
// must reference declared concepts; constraints must have at least one example).
func Validate(m *Map) []error {
	var errs []error
	if strings.Count(m.Version, ".") != 2 {
		errs = append(errs, fmt.Errorf("version %q: expected semver (X.Y.Z)", m.Version))
	}
	names := map[string]bool{}
	for i, c := range m.Concepts {
		if !nameRe.MatchString(c.Name) {
			errs = append(errs, fmt.Errorf("concepts[%d].name %q: must be PascalCase", i, c.Name))
		}
		if !validKind[c.Kind] {
			errs = append(errs, fmt.Errorf("concepts[%d].kind %q: invalid", i, c.Kind))
		}
		if !validRepo[c.Owner] {
			errs = append(errs, fmt.Errorf("concepts[%d].owner %q: unknown repo", i, c.Owner))
		}
		if c.Description == "" {
			errs = append(errs, fmt.Errorf("concepts[%d].description: required", i))
		}
		if len(c.Examples) == 0 {
			errs = append(errs, fmt.Errorf("concepts[%d].examples: at least one required", i))
		}
		if names[c.Name] {
			errs = append(errs, fmt.Errorf("concepts: duplicate name %q", c.Name))
		}
		names[c.Name] = true
	}
	for i, r := range m.Relationships {
		if !names[r.From] {
			errs = append(errs, fmt.Errorf("relationships[%d].from %q: not a declared concept", i, r.From))
		}
		if !names[r.To] {
			errs = append(errs, fmt.Errorf("relationships[%d].to %q: not a declared concept", i, r.To))
		}
		if !validRel[r.Kind] {
			errs = append(errs, fmt.Errorf("relationships[%d].kind %q: invalid", i, r.Kind))
		}
		if r.Example == "" {
			errs = append(errs, fmt.Errorf("relationships[%d].example: required", i))
		}
	}
	cids := map[string]bool{}
	for i, c := range m.Constraints {
		if !cidRe.MatchString(c.ID) {
			errs = append(errs, fmt.Errorf("constraints[%d].id %q: must match ^C-[0-9]{3}$", i, c.ID))
		}
		if cids[c.ID] {
			errs = append(errs, fmt.Errorf("constraints: duplicate id %q", c.ID))
		}
		cids[c.ID] = true
		if c.Rule == "" {
			errs = append(errs, fmt.Errorf("constraints[%d].rule: required", i))
		}
		if c.Rationale == "" {
			errs = append(errs, fmt.Errorf("constraints[%d].rationale: required", i))
		}
		if len(c.Examples) == 0 {
			errs = append(errs, fmt.Errorf("constraints[%d].examples: at least one required", i))
		}
		if c.GuardCandidate != nil && !validGuardStatus[c.GuardCandidate.Status] {
			errs = append(errs, fmt.Errorf("constraints[%d].guard_candidate.status %q: invalid", i, c.GuardCandidate.Status))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// Query returns the concept matching name (case-insensitive) and any
// relationships or constraints that mention it.
func (m *Map) Query(name string) (*Concept, []Relationship, []Constraint) {
	var matched *Concept
	for i, c := range m.Concepts {
		if strings.EqualFold(c.Name, name) {
			matched = &m.Concepts[i]
			break
		}
	}
	var rels []Relationship
	for _, r := range m.Relationships {
		if strings.EqualFold(r.From, name) || strings.EqualFold(r.To, name) {
			rels = append(rels, r)
		}
	}
	var cons []Constraint
	for _, c := range m.Constraints {
		if strings.Contains(strings.ToLower(c.Rule), strings.ToLower(name)) ||
			strings.Contains(strings.ToLower(c.Rationale), strings.ToLower(name)) {
			cons = append(cons, c)
		}
	}
	if matched == nil && len(rels) == 0 && len(cons) == 0 {
		return nil, nil, nil
	}
	return matched, rels, cons
}

var _ = errors.New // keep import for future use
