// Package decision loads, validates, lists, and supersedes decision records.
// Records live under decisions/YYYY/MM/DD/<id>.decision.riido.json and conform
// to decisions/schema.riido.json.
package decision

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

// Record is the in-memory shape of a decision record. Field tags mirror the
// JSON Schema at decisions/schema.riido.json. Keep in sync when the schema
// changes (and record a decision when it does).
type Record struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	Owner              string     `json:"owner"`
	Status             string     `json:"status"`
	Weak               bool       `json:"weak,omitempty"`
	Scope              Scope      `json:"scope"`
	Source             string     `json:"source"`
	Evidence           []Evidence `json:"evidence"`
	AffectedRepos      []string   `json:"affected_repos"`
	SsotOwner          string     `json:"ssot_owner"`
	GeneratedArtifacts []GenArt   `json:"generated_artifacts"`
	Guards             []Guard    `json:"guards"`
	Examples           []string   `json:"examples"`
	Counterexamples    []string   `json:"counterexamples"`
	Supersedes         []string   `json:"supersedes,omitempty"`
	SupersededBy       *string    `json:"superseded_by,omitempty"`
	CreatedAt          string     `json:"created_at"`
	AcceptedAt         *string    `json:"accepted_at,omitempty"`
	Notes              string     `json:"notes,omitempty"`

	// Path is populated by Load; not serialized.
	Path string `json:"-"`
}

type Scope struct {
	BoundedContexts []string `json:"bounded_contexts"`
	Areas           []string `json:"areas"`
}

type Evidence struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
	Note string `json:"note,omitempty"`
}

type GenArt struct {
	Repo string `json:"repo"`
	Path string `json:"path"`
	From string `json:"from"`
}

type Guard struct {
	Kind   string `json:"kind"`
	Ref    string `json:"ref"`
	Status string `json:"status,omitempty"`
}

var (
	idRe     = regexp.MustCompile(`^[0-9]{3}-[a-z0-9][a-z0-9-]*$`)
	validSt  = map[string]bool{"proposed": true, "accepted": true, "superseded": true, "rejected": true}
	validSrc = map[string]bool{"top_down": true, "bottom_up": true, "generated": true, "imported": true}
	validRep = map[string]bool{
		"agent-cluster-contracts": true,
		"agent-cluster-backend":   true,
		"agent-cluster-frontend":  true,
	}
	validAreas = map[string]bool{
		"dsl": true, "ir": true, "concept-map": true, "ssot-dep-map": true,
		"tooling": true, "ci": true, "security": true, "backend": true,
		"frontend": true, "deployment": true, "governance": true,
	}
	// D036: bounded_contexts enum, mirroring decisions/schema.riido.json.
	// Until D036 BCs were unbounded — typos and ad-hoc names went undetected
	// because the schema lacked an enum constraint and decision.go didn't
	// validate them either. Initial set is what's been in use across D001..D035.
	validBoundedContexts = map[string]bool{
		"platform": true, "work-item": true,
	}
	validEvKind = map[string]bool{
		"file": true, "url": true, "pr": true, "issue": true, "ci-run": true,
		"log": true, "incident": true, "conversation": true, "decision": true,
	}
	validGuardKind = map[string]bool{
		"go-tool": true, "github-action": true, "pre-commit-hook": true,
		"test": true, "schema": true, "generated-check": true, "manual-review": true,
	}
)

// Load reads and parses (but does not validate) one decision record.
func Load(path string) (*Record, error) {
	r := &Record{Path: path}
	if err := jsonutil.ReadFile(path, r); err != nil {
		return nil, err
	}
	return r, nil
}

// LoadAll walks decisions/ under root and returns every parsed record, sorted by id.
func LoadAll(root string) ([]*Record, error) {
	dir := filepath.Join(root, "decisions")
	var out []*Record
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".decision.riido.json") {
			return nil
		}
		r, err := Load(path)
		if err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Validate checks one record against the schema rules.
func Validate(r *Record) []error {
	var errs []error
	if !idRe.MatchString(r.ID) {
		errs = append(errs, fmt.Errorf("id %q: must match ^[0-9]{3}-[a-z0-9][a-z0-9-]*$", r.ID))
	}
	if r.Title == "" {
		errs = append(errs, errors.New("title: required"))
	}
	if r.Owner == "" {
		errs = append(errs, errors.New("owner: required"))
	}
	if !validSt[r.Status] {
		errs = append(errs, fmt.Errorf("status %q: must be proposed|accepted|superseded|rejected", r.Status))
	}
	if !validSrc[r.Source] {
		errs = append(errs, fmt.Errorf("source %q: must be top_down|bottom_up|generated|imported", r.Source))
	}
	if len(r.Evidence) == 0 {
		errs = append(errs, errors.New("evidence: at least one entry required"))
	}
	for i, e := range r.Evidence {
		if !validEvKind[e.Kind] {
			errs = append(errs, fmt.Errorf("evidence[%d].kind %q: invalid", i, e.Kind))
		}
		if e.Ref == "" {
			errs = append(errs, fmt.Errorf("evidence[%d].ref: required", i))
		}
	}
	if len(r.AffectedRepos) == 0 {
		errs = append(errs, errors.New("affected_repos: at least one entry required"))
	}
	for _, a := range r.AffectedRepos {
		if !validRep[a] {
			errs = append(errs, fmt.Errorf("affected_repos: unknown repo %q", a))
		}
	}
	if !validRep[r.SsotOwner] {
		errs = append(errs, fmt.Errorf("ssot_owner %q: must be one of the three known repos", r.SsotOwner))
	}
	for _, bc := range r.Scope.BoundedContexts {
		if !validBoundedContexts[bc] {
			errs = append(errs, fmt.Errorf("scope.bounded_contexts: unknown bounded context %q (must be one of: platform, work-item — extending the enum requires a decision that updates both decisions/schema.riido.json and internal/decision/decision.go)", bc))
		}
	}
	for _, a := range r.Scope.Areas {
		if !validAreas[a] {
			errs = append(errs, fmt.Errorf("scope.areas: unknown area %q", a))
		}
	}
	for i, g := range r.Guards {
		if !validGuardKind[g.Kind] {
			errs = append(errs, fmt.Errorf("guards[%d].kind %q: invalid", i, g.Kind))
		}
		if g.Ref == "" {
			errs = append(errs, fmt.Errorf("guards[%d].ref: required", i))
		}
	}
	if len(r.Examples) == 0 {
		errs = append(errs, errors.New("examples: at least one entry required"))
	}
	if r.CreatedAt == "" {
		errs = append(errs, errors.New("created_at: required (YYYY-MM-DD)"))
	}
	if r.Status == "superseded" && (r.SupersededBy == nil || *r.SupersededBy == "") {
		errs = append(errs, errors.New("superseded_by: required when status=superseded"))
	}
	return errs
}

// PathFor returns the canonical path for a decision record with id and dated dir.
// date is YYYY-MM-DD. root is the contracts repo root.
func PathFor(root, date, id string) (string, error) {
	if len(date) != 10 || date[4] != '-' || date[7] != '-' {
		return "", fmt.Errorf("date %q: expected YYYY-MM-DD", date)
	}
	y, m, d := date[:4], date[5:7], date[8:10]
	return filepath.Join(root, "decisions", y, m, d, id+".decision.riido.json"), nil
}

// Save writes a record to disk, creating parent directories as needed.
func Save(r *Record, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return jsonutil.WriteFile(path, r)
}
