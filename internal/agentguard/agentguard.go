// Package agentguard enforces the per-role path allowlists declared in
// agent-roles.riido.json. It does not run git itself — callers pass in the
// list of changed paths (from `git diff --name-only`, a GitHub event payload,
// or a synthetic list for tests).
//
// Rule, for each role:
//   - every changed path must match at least one allowed_paths glob
//   - no changed path may match any forbidden_paths glob (forbidden wins)
//   - if pr_isolation.candidate_only is true, none of the changed paths may
//     match any path declared as forbidden for this role (already enforced)
//     AND none may fall outside allowed_paths
package agentguard

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

type Roles struct {
	Version string `json:"version"`
	Owner   string `json:"owner"`
	Roles   []Role `json:"roles"`
}

type Role struct {
	ID                string      `json:"id"`
	Label             string      `json:"label"`
	Description       string      `json:"description"`
	Decision          string      `json:"decision,omitempty"`
	AllowedPaths      []string    `json:"allowed_paths"`
	ForbiddenPaths    []string    `json:"forbidden_paths"`
	PRIsolation       PRIsolation `json:"pr_isolation"`
	MaxFilesPerPR     int         `json:"max_files_per_pr,omitempty"`
	AutoMergeEligible bool        `json:"auto_merge_eligible,omitempty"`
	AutoMergePaths    []string    `json:"auto_merge_paths,omitempty"`
}

type PRIsolation struct {
	CandidateOnly bool   `json:"candidate_only"`
	GuardOnly     bool   `json:"guard_only"`
	Rationale     string `json:"rationale,omitempty"`
}

// Load reads root/agent-roles.riido.json.
func Load(root string) (*Roles, error) {
	r := &Roles{}
	if err := jsonutil.ReadFile(filepath.Join(root, "agent-roles.riido.json"), r); err != nil {
		return nil, err
	}
	return r, nil
}

// Lookup returns the role with the given id.
func (r *Roles) Lookup(id string) *Role {
	for i := range r.Roles {
		if r.Roles[i].ID == id {
			return &r.Roles[i]
		}
	}
	return nil
}

// Violation is one rejected path with the reason.
type Violation struct {
	Path   string
	Reason string
}

// Check applies the role's rules to the changed paths.
// Returns an empty slice when allowed.
func Check(role *Role, changed []string) []Violation {
	var out []Violation
	if role == nil {
		return []Violation{{Path: "", Reason: "no role provided"}}
	}
	if role.MaxFilesPerPR > 0 && len(changed) > role.MaxFilesPerPR {
		out = append(out, Violation{
			Path:   "",
			Reason: fmt.Sprintf("PR touches %d files, role %q allows at most %d", len(changed), role.ID, role.MaxFilesPerPR),
		})
	}
	for _, p := range changed {
		p = filepath.ToSlash(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		// Forbidden wins.
		if forb, hit := matchAny(p, role.ForbiddenPaths); hit {
			out = append(out, Violation{Path: p, Reason: "matches forbidden_paths pattern " + forb})
			continue
		}
		if _, hit := matchAny(p, role.AllowedPaths); !hit {
			out = append(out, Violation{Path: p, Reason: "does not match any allowed_paths pattern for role " + role.ID})
		}
	}
	return out
}

// matchAny returns the first pattern that matches p, and whether any matched.
func matchAny(p string, patterns []string) (string, bool) {
	for _, pat := range patterns {
		if matchGlob(pat, p) {
			return pat, true
		}
	}
	return "", false
}

// matchGlob is a minimal ** + * + ? glob matcher. ** matches any number of
// path components (including zero). * matches anything except '/'. ? matches
// a single character except '/'. Patterns are compared as forward-slash paths.
func matchGlob(pattern, name string) bool {
	pattern = filepath.ToSlash(pattern)
	name = filepath.ToSlash(name)
	return globMatch(pattern, name)
}

func globMatch(pat, name string) bool {
	for {
		switch {
		case pat == "" && name == "":
			return true
		case pat == "":
			return false
		case strings.HasPrefix(pat, "**/"):
			// '**/x' matches 'x' OR consume one segment of name and retry
			if globMatch(pat[3:], name) {
				return true
			}
			i := strings.Index(name, "/")
			if i < 0 {
				return false
			}
			name = name[i+1:]
		case pat == "**":
			return true
		case strings.HasPrefix(pat, "*"):
			rest := pat[1:]
			for i := 0; i <= len(name); i++ {
				if i > 0 && name[i-1] == '/' {
					return false
				}
				if globMatch(rest, name[i:]) {
					return true
				}
			}
			return false
		case strings.HasPrefix(pat, "?"):
			if name == "" || name[0] == '/' {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		default:
			if name == "" || pat[0] != name[0] {
				return false
			}
			pat = pat[1:]
			name = name[1:]
		}
	}
}
