// Package agentguard enforces the per-role path allowlists declared in
// agent-roles.riido.json. It does not run git itself — callers pass in the
// list of changed paths (from `git diff --name-only`, a GitHub event payload,
// or a synthetic list for tests).
//
// Rules, per role:
//   - if MaxFilesPerPR > 0 and the diff exceeds it, emit a max_files
//     violation with a split-PR hint;
//   - for each path: if it matches any forbidden_paths glob, emit a
//     forbidden violation (and skip the not-allowed check — forbidden wins);
//   - else if it does not match any allowed_paths glob, emit a not_allowed
//     violation, attempt to suggest the closest allowed pattern, and record
//     any missing path prefix that would have made it match;
//   - if many not_allowed violations share the same missing prefix, emit a
//     single prefix_drift hint at the result level.
package agentguard

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

type Roles struct {
	Version string `json:"version"`
	Owner   string `json:"owner"`
	Roles   []Role `json:"roles"`
}

type Role struct {
	ID                string           `json:"id"`
	Label             string           `json:"label"`
	Description       string           `json:"description"`
	Decision          string           `json:"decision,omitempty"`
	AllowedPaths      []string         `json:"allowed_paths"`
	ForbiddenPaths    []string         `json:"forbidden_paths"`
	PRIsolation       PRIsolation      `json:"pr_isolation"`
	MaxFilesPerPR     int              `json:"max_files_per_pr,omitempty"`
	AutoMergeEligible bool             `json:"auto_merge_eligible,omitempty"`
	AutoMergePaths    []string         `json:"auto_merge_paths,omitempty"`
	MergeAuthority    *MergeAuthority  `json:"merge_authority,omitempty"`
}

// MergeAuthority records the per-role bounded-merge grant from decision 010.
type MergeAuthority struct {
	Granted   bool   `json:"granted"`
	Decision  string `json:"decision"`
	Rationale string `json:"rationale"`
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

var (
	semverRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	roleIDRe = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	decIDRe  = regexp.MustCompile(`^[0-9]{3}-[a-z0-9][a-z0-9-]*$`)
)

// ValidateRoles mirrors agent-roles.schema.json in Go. Returns one error per
// rule violation. An empty return value means the SSOT artifact is shaped
// correctly. Decision 009 introduced this — previously the only check was
// "JSON parses into Go struct, extras ignored", which silently accepted typos.
func ValidateRoles(r *Roles) []error {
	if r == nil {
		return []error{fmt.Errorf("roles: nil")}
	}
	var errs []error
	if !semverRe.MatchString(r.Version) {
		errs = append(errs, fmt.Errorf("version %q: must match X.Y.Z", r.Version))
	}
	if r.Owner != "agent-cluster-contracts" {
		errs = append(errs, fmt.Errorf("owner %q: must be agent-cluster-contracts", r.Owner))
	}
	if len(r.Roles) == 0 {
		errs = append(errs, fmt.Errorf("roles: at least one role required"))
	}
	seenIDs := map[string]bool{}
	for i, role := range r.Roles {
		if !roleIDRe.MatchString(role.ID) {
			errs = append(errs, fmt.Errorf("roles[%d].id %q: must match ^[a-z][a-z0-9-]*$", i, role.ID))
		}
		if seenIDs[role.ID] {
			errs = append(errs, fmt.Errorf("roles[%d].id %q: duplicate", i, role.ID))
		}
		seenIDs[role.ID] = true
		if role.Label == "" {
			errs = append(errs, fmt.Errorf("roles[%d] (%s): label required", i, role.ID))
		}
		if role.Description == "" {
			errs = append(errs, fmt.Errorf("roles[%d] (%s): description required", i, role.ID))
		}
		if role.Decision != "" && !decIDRe.MatchString(role.Decision) {
			errs = append(errs, fmt.Errorf("roles[%d] (%s).decision %q: must match NNN-slug", i, role.ID, role.Decision))
		}
		if role.AllowedPaths == nil {
			errs = append(errs, fmt.Errorf("roles[%d] (%s): allowed_paths required (use empty array, never null)", i, role.ID))
		}
		for j, p := range role.AllowedPaths {
			if strings.TrimSpace(p) == "" {
				errs = append(errs, fmt.Errorf("roles[%d] (%s).allowed_paths[%d]: empty string", i, role.ID, j))
			}
		}
		if role.ForbiddenPaths == nil {
			errs = append(errs, fmt.Errorf("roles[%d] (%s): forbidden_paths required (use empty array, never null)", i, role.ID))
		}
		for j, p := range role.ForbiddenPaths {
			if strings.TrimSpace(p) == "" {
				errs = append(errs, fmt.Errorf("roles[%d] (%s).forbidden_paths[%d]: empty string", i, role.ID, j))
			}
		}
		if role.MaxFilesPerPR < 0 {
			errs = append(errs, fmt.Errorf("roles[%d] (%s): max_files_per_pr must be ≥ 0", i, role.ID))
		}
	}
	return errs
}

// ViolationKind classifies a violation.
type ViolationKind string

const (
	KindMaxFiles   ViolationKind = "max_files"
	KindForbidden  ViolationKind = "forbidden"
	KindNotAllowed ViolationKind = "not_allowed"
)

// Violation is one rejected path (or one whole-PR violation, with empty Path).
type Violation struct {
	Path string        `json:"path"`
	Kind ViolationKind `json:"kind"`
	// Reason is a human-readable one-liner suitable for direct CI log output.
	Reason string `json:"reason"`
	// MatchedPattern: for KindForbidden, the forbidden pattern that matched.
	MatchedPattern string `json:"matched_pattern,omitempty"`
	// ClosestAllowed: for KindNotAllowed, the allowed pattern that came nearest
	// to matching this path (by leading-segment match count). Empty if no
	// pattern shares any prefix with the path.
	ClosestAllowed string `json:"closest_allowed,omitempty"`
	// MissingPrefix: for KindNotAllowed, the leading prefix the path appears to
	// be missing in order to match ClosestAllowed (e.g. "contracts"). Empty if
	// no prefix would have helped.
	MissingPrefix string `json:"missing_prefix,omitempty"`
}

// CheckResult is the full result of Check, including per-path violations and
// global hints (like prefix_drift) that describe the diff as a whole.
type CheckResult struct {
	RoleID     string      `json:"role"`
	OK         bool        `json:"ok"`
	Files      []string    `json:"files"`
	Violations []Violation `json:"violations"`
	Hints      []string    `json:"hints,omitempty"`
}

// MergeResult is the verdict from MergeCheck (decision 010). Stricter than
// Check: a role may be allowed to *write* a path (allowed_paths) but not
// allowed to *auto-merge* it (auto_merge_paths). The dumb-agent reads
// Allowed and either calls `gh pr merge` or waits for designer review.
type MergeResult struct {
	RoleID   string   `json:"role"`
	Allowed  bool     `json:"allowed"`
	Status   string   `json:"status"`   // "merge_allowed" | "merge_blocked"
	Reasons  []string `json:"reasons"`  // empty when Allowed
	Files    []string `json:"files"`
}

// Check applies the role's rules to the changed paths and returns the result.
func Check(role *Role, changed []string) CheckResult {
	res := CheckResult{Files: changed}
	if role == nil {
		res.Violations = append(res.Violations, Violation{
			Kind:   KindMaxFiles, // bucket as max_files-style PR-level error
			Reason: "no role provided",
		})
		return res
	}
	res.RoleID = role.ID

	if role.MaxFilesPerPR > 0 && len(changed) > role.MaxFilesPerPR {
		res.Violations = append(res.Violations, Violation{
			Kind: KindMaxFiles,
			Reason: fmt.Sprintf(
				"PR touches %d files, role %q allows at most %d. "+
					"Hint: split into multiple PRs of ≤ %d files each, grouped by task type "+
					"(positive fixtures, negative fixtures, fuzz corpus, guard-candidates).",
				len(changed), role.ID, role.MaxFilesPerPR, role.MaxFilesPerPR,
			),
		})
	}

	for _, raw := range changed {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if pat, hit := matchAny(p, role.ForbiddenPaths); hit {
			res.Violations = append(res.Violations, Violation{
				Path:           p,
				Kind:           KindForbidden,
				MatchedPattern: pat,
				Reason:         fmt.Sprintf("FORBIDDEN: matches forbidden_paths pattern %q", pat),
			})
			continue
		}
		if _, hit := matchAny(p, role.AllowedPaths); hit {
			continue
		}
		closest, missing := closestAllowed(p, role.AllowedPaths)
		v := Violation{
			Path:           p,
			Kind:           KindNotAllowed,
			ClosestAllowed: closest,
			MissingPrefix:  missing,
		}
		switch {
		case missing != "":
			v.Reason = fmt.Sprintf(
				"not allowed for role %s; would match %q if prefixed with %q",
				role.ID, closest, missing,
			)
		case closest != "":
			v.Reason = fmt.Sprintf(
				"not allowed for role %s; closest allowed pattern is %q",
				role.ID, closest,
			)
		default:
			v.Reason = fmt.Sprintf(
				"not allowed for role %s; no allowed_paths pattern matches or comes near",
				role.ID,
			)
		}
		res.Violations = append(res.Violations, v)
	}

	if drift := detectPrefixDrift(res.Violations); drift != "" {
		res.Hints = append(res.Hints,
			fmt.Sprintf(
				"prefix_drift: %d not_allowed paths would match if prefixed with %q. "+
					"This usually means agent-roles.riido.json was written with a different "+
					"path layout than the diff producer (e.g. monorepo vs single-repo CI).",
				countNotAllowedWithMissing(res.Violations, drift), drift,
			),
		)
	}

	res.OK = len(res.Violations) == 0
	return res
}

// MergeCheck decides whether a role may merge a PR with the given file list.
// Decision 010 introduced this — it's strictly stricter than Check:
//
//   - Every changed file must match at least one auto_merge_paths glob
//     (NOT allowed_paths). If a role has no auto_merge_paths, MergeCheck
//     refuses every PR.
//   - No changed file may match any forbidden_paths glob (forbidden wins,
//     same as Check).
//   - File count must be <= MaxFilesPerPR.
//
// The dumb-agent reads Allowed before calling `gh pr merge`. CI's required
// checks (verify, drift, scan, etc.) are orthogonal — they're verified by
// the merge platform; MergeCheck only validates the diff itself.
func MergeCheck(role *Role, changed []string) MergeResult {
	res := MergeResult{Files: changed}
	if role == nil {
		res.Allowed = false
		res.Status = "merge_blocked"
		res.Reasons = []string{"no role provided"}
		return res
	}
	res.RoleID = role.ID

	if len(role.AutoMergePaths) == 0 {
		res.Allowed = false
		res.Status = "merge_blocked"
		res.Reasons = []string{
			fmt.Sprintf("role %q has no auto_merge_paths — bounded merge authority not granted", role.ID),
		}
		return res
	}

	if role.MaxFilesPerPR > 0 && len(changed) > role.MaxFilesPerPR {
		res.Reasons = append(res.Reasons,
			fmt.Sprintf("PR touches %d files; role %q allows at most %d for auto-merge", len(changed), role.ID, role.MaxFilesPerPR))
	}

	for _, raw := range changed {
		p := filepath.ToSlash(strings.TrimSpace(raw))
		if p == "" {
			continue
		}
		if pat, hit := matchAny(p, role.ForbiddenPaths); hit {
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("FORBIDDEN: %s matches forbidden_paths pattern %q", p, pat))
			continue
		}
		if _, hit := matchAny(p, role.AutoMergePaths); !hit {
			res.Reasons = append(res.Reasons,
				fmt.Sprintf("not auto-mergeable: %s outside auto_merge_paths for role %s (writable but designer must review)", p, role.ID))
		}
	}

	if len(res.Reasons) == 0 {
		res.Allowed = true
		res.Status = "merge_allowed"
	} else {
		res.Allowed = false
		res.Status = "merge_blocked"
	}
	return res
}

// detectPrefixDrift returns the most common MissingPrefix across not_allowed
// violations when it accounts for at least two violations and a majority of
// not_allowed violations. Returns "" when no drift signal is clear.
func detectPrefixDrift(vs []Violation) string {
	counts := map[string]int{}
	total := 0
	for _, v := range vs {
		if v.Kind != KindNotAllowed {
			continue
		}
		total++
		if v.MissingPrefix != "" {
			counts[v.MissingPrefix]++
		}
	}
	best := ""
	bestN := 0
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic tie-break
	for _, k := range keys {
		if counts[k] > bestN {
			best = k
			bestN = counts[k]
		}
	}
	if bestN < 2 {
		return ""
	}
	if bestN*2 < total {
		// Less than half — too noisy to call drift.
		return ""
	}
	return best
}

func countNotAllowedWithMissing(vs []Violation, prefix string) int {
	n := 0
	for _, v := range vs {
		if v.Kind == KindNotAllowed && v.MissingPrefix == prefix {
			n++
		}
	}
	return n
}

// closestAllowed scans allowed patterns and returns the pattern whose literal
// prefix shares the most leading path segments with `path`. If the best match
// requires skipping some leading segments OF THE PATTERN (i.e. the path lacks
// a prefix that the pattern has), the dropped segments are returned as
// missingPrefix.
//
// Example:
//
//	path     = "fixtures/positive/x.json"
//	patterns = ["contracts/fixtures/positive/**", "fuzz/corpus/**"]
//	→ closest="contracts/fixtures/positive/**", missing="contracts"
func closestAllowed(path string, patterns []string) (closest, missing string) {
	pathSegs := strings.Split(path, "/")
	bestScore := 0
	for _, pat := range patterns {
		score, miss := segmentDiff(pat, pathSegs)
		if score > bestScore {
			bestScore = score
			closest = pat
			missing = miss
		}
	}
	if bestScore == 0 {
		return "", ""
	}
	return closest, missing
}

// segmentDiff returns the number of leading pattern-prefix segments that match
// some prefix of pathSegs after dropping `skip` leading pattern segments. It
// picks the best (skip, matched) combination such that matched is maximized
// and ties go to smaller skip. When skip > 0, the dropped pattern segments are
// returned as missingPrefix.
func segmentDiff(pattern string, pathSegs []string) (matched int, missingPrefix string) {
	patSegs := strings.Split(patternPrefix(pattern), "/")
	bestMatched := 0
	bestSkip := -1
	for skip := 0; skip < len(patSegs); skip++ {
		m := 0
		for i := skip; i < len(patSegs); i++ {
			j := i - skip
			if j >= len(pathSegs) || patSegs[i] != pathSegs[j] {
				break
			}
			m++
		}
		if m > bestMatched {
			bestMatched = m
			bestSkip = skip
		}
	}
	if bestMatched == 0 {
		return 0, ""
	}
	if bestSkip > 0 {
		missingPrefix = strings.Join(patSegs[:bestSkip], "/")
	}
	return bestMatched, missingPrefix
}

// patternPrefix returns the literal leading portion of a glob (everything up
// to the first wildcard character). Trailing slashes are stripped.
func patternPrefix(pattern string) string {
	idx := strings.IndexAny(pattern, "*?")
	if idx < 0 {
		return strings.TrimRight(pattern, "/")
	}
	return strings.TrimRight(pattern[:idx], "/")
}

// matchAny returns the first pattern that matches p, and whether any matched.
func matchAny(p string, patterns []string) (string, bool) {
	return MatchAny(p, patterns)
}

// MatchAny returns the first pattern in patterns that matches p (via
// MatchGlob), and true; or "", false if no pattern matches. Exported
// for reuse by ssotdeps cross-check (D032) which needs to verify
// dumb-agent forbidden_paths covers every SSOT data+schema pair
// without re-implementing the matcher.
func MatchAny(p string, patterns []string) (string, bool) {
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
