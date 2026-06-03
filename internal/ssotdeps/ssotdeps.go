// Package ssotdeps loads and verifies the SSOT dependency map. It checks that
// every declared artifact, schema, consumer, and CI workflow actually exists on
// disk relative to the contracts repo root.
//
// Verify has two modes:
//   - ModeFull (default): also checks sibling repos (backend, frontend) when
//     they are present in the parent directory. Used by smart-agent local dev
//     and by full cross-repo CI.
//   - ModeLocal: ignores sibling repos entirely, even if their directories
//     exist. Used by the dumb-agent probe preflight so its baseline answer is
//     deterministic regardless of what happens to be in the parent directory.
package ssotdeps

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/agentguard"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/conceptmap"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
)

type Map struct {
	Version         string             `json:"version"`
	Owner           string             `json:"owner"`
	Description     string             `json:"description"`
	SsotArtifacts   []SsotArtifact     `json:"ssot_artifacts"`
	GenerationLinks []GenerationLink   `json:"generation_links"`
	ConsumptionLinks []ConsumptionLink `json:"consumption_links"`
	CIGates         []CIGate           `json:"ci_gates"`
	Pending         []string           `json:"pending"`
}

type SsotArtifact struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Path        string   `json:"path"`
	Schema      *string  `json:"schema"`
	MirroredIn  []string `json:"mirrored_in,omitempty"`
	OwnedBy     string   `json:"owned_by"`
}

type GenerationLink struct {
	From         string `json:"from"`
	Emitter      string `json:"emitter"`
	To           string `json:"to"`
	ConsumerRepo string `json:"consumer_repo,omitempty"`
	Guard        string `json:"guard,omitempty"`
}

type ConsumptionLink struct {
	SSOT         string `json:"ssot"`
	ConsumerRepo string `json:"consumer_repo"`
	ConsumerPath string `json:"consumer_path"`
	Via          string `json:"via,omitempty"`
	Guard        string `json:"guard,omitempty"`
	// Planned: the consumer file does not exist yet (generator hasn't run in
	// the sibling repo). Verify skips the existence check so the dep map can
	// record the dependency before the file lands.
	Planned bool `json:"planned,omitempty"`
}

type CIGate struct {
	Repo     string   `json:"repo"`
	Workflow string   `json:"workflow"`
	Verifies []string `json:"verifies"`
	// Planned means this gate is declared but not yet wired up. Verify skips
	// the file-existence check for planned gates so the dep map can record
	// intent without false-positive failures. Flip to false when the file
	// lands.
	Planned bool `json:"planned,omitempty"`
}

// Load reads root/ssot-dependency-map.riido.json.
func Load(root string) (*Map, error) {
	m := &Map{}
	if err := jsonutil.ReadFile(filepath.Join(root, "ssot-dependency-map.riido.json"), m); err != nil {
		return nil, err
	}
	return m, nil
}

var (
	semverRe    = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	idRe        = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	validKinds  = map[string]bool{
		"decision": true, "concept-map": true, "ssot-dependency-map": true,
		"research": true, "agent-roles": true, "dsl-source": true,
		"dsl-emitter": true, "ir": true, "code-generator": true,
		"verifier": true, "doc": true, "report-area": true, "schema": true,
	}
)

// ValidateMap mirrors ssot-dependency-map.schema.json in Go (decision 011).
// Returns one error per rule violation; empty slice means OK. Catches typos
// and structural issues that Load + plain JSON unmarshal silently accept.
func ValidateMap(m *Map) []error {
	if m == nil {
		return []error{fmt.Errorf("ssot-dep-map: nil")}
	}
	var errs []error
	if !semverRe.MatchString(m.Version) {
		errs = append(errs, fmt.Errorf("version %q: must match X.Y.Z", m.Version))
	}
	if m.Owner != "agent-cluster-contracts" {
		errs = append(errs, fmt.Errorf("owner %q: must be agent-cluster-contracts", m.Owner))
	}
	if len(m.SsotArtifacts) == 0 {
		errs = append(errs, fmt.Errorf("ssot_artifacts: at least one artifact required"))
	}
	seenIDs := map[string]bool{}
	for i, a := range m.SsotArtifacts {
		if !idRe.MatchString(a.ID) {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d].id %q: must match ^[a-z][a-z0-9-]*$", i, a.ID))
		}
		if seenIDs[a.ID] {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d].id %q: duplicate", i, a.ID))
		}
		seenIDs[a.ID] = true
		if !validKinds[a.Kind] {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (%s).kind %q: invalid (see schema enum)", i, a.ID, a.Kind))
		}
		if a.Path == "" {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (%s).path: required", i, a.ID))
		}
		if a.OwnedBy == "" {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (%s).owned_by: required", i, a.ID))
		}
	}
	for i, l := range m.GenerationLinks {
		if l.From == "" {
			errs = append(errs, fmt.Errorf("generation_links[%d].from: required", i))
		}
		if l.Emitter == "" {
			errs = append(errs, fmt.Errorf("generation_links[%d].emitter: required", i))
		}
		if l.To == "" {
			errs = append(errs, fmt.Errorf("generation_links[%d].to: required", i))
		}
	}
	for i, l := range m.ConsumptionLinks {
		if l.SSOT == "" {
			errs = append(errs, fmt.Errorf("consumption_links[%d].ssot: required", i))
		} else if !seenIDs[l.SSOT] {
			errs = append(errs, fmt.Errorf("consumption_links[%d].ssot %q: references unknown artifact", i, l.SSOT))
		}
		if l.ConsumerRepo == "" {
			errs = append(errs, fmt.Errorf("consumption_links[%d].consumer_repo: required", i))
		}
		if l.ConsumerPath == "" {
			errs = append(errs, fmt.Errorf("consumption_links[%d].consumer_path: required", i))
		}
	}
	for i, g := range m.CIGates {
		if g.Repo == "" {
			errs = append(errs, fmt.Errorf("ci_gates[%d].repo: required", i))
		}
		if g.Workflow == "" {
			errs = append(errs, fmt.Errorf("ci_gates[%d].workflow: required", i))
		}
		if len(g.Verifies) == 0 {
			errs = append(errs, fmt.Errorf("ci_gates[%d].verifies: at least one entry required (artifact id or free-form concern description)", i))
		}
		for j, v := range g.Verifies {
			if v == "" {
				errs = append(errs, fmt.Errorf("ci_gates[%d].verifies[%d]: empty string", i, j))
			}
		}
	}
	return errs
}

// Mode controls how much of the dep map is checked.
type Mode int

const (
	// ModeFull verifies everything, including sibling-repo consumer paths and
	// CI gates in sibling repos when those repos are present locally. This is
	// the default and what cross-repo CI uses.
	ModeFull Mode = iota
	// ModeLocal verifies only contracts-repo-owned artifacts and gates. It
	// never reads sibling-repo state, even if backend/ or frontend/ are
	// present. This is what the dumb-agent probe preflight uses so the
	// baseline answer cannot be polluted by an incomplete sibling checkout.
	ModeLocal
)

func (m Mode) String() string {
	switch m {
	case ModeLocal:
		return "local"
	default:
		return "full"
	}
}

// Verify checks that referenced files exist. Returns one error per failure
// (empty slice means OK). See Mode for the cross-repo behavior.
func Verify(root string, m *Map, mode Mode) []error {
	var errs []error
	ids := map[string]bool{}
	for i, a := range m.SsotArtifacts {
		ids[a.ID] = true
		if a.Path == "" {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d].path: required", i))
			continue
		}
		if err := mustExist(root, a.Path); err != nil {
			errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (id=%s): %w", i, a.ID, err))
		}
		if a.Schema != nil && *a.Schema != "" {
			if err := mustExist(root, *a.Schema); err != nil {
				errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (id=%s) schema: %w", i, a.ID, err))
			}
		}
		for _, m := range a.MirroredIn {
			if err := mustExist(root, m); err != nil {
				errs = append(errs, fmt.Errorf("ssot_artifacts[%d] (id=%s) mirror: %w", i, a.ID, err))
			}
		}
	}
	for i, l := range m.ConsumptionLinks {
		if !ids[l.SSOT] {
			errs = append(errs, fmt.Errorf("consumption_links[%d].ssot %q: not in ssot_artifacts", i, l.SSOT))
		}
		if l.Planned {
			continue
		}
		if l.ConsumerRepo == "agent-cluster-contracts" {
			if err := mustExist(root, l.ConsumerPath); err != nil {
				errs = append(errs, fmt.Errorf("consumption_links[%d] consumer_path: %w", i, err))
			}
			continue
		}
		// Sibling repo. In ModeLocal we never look at siblings — that's the
		// point of local mode: deterministic baseline regardless of sibling
		// checkout state.
		if mode == ModeLocal {
			continue
		}
		sibling := siblingRoot(root, l.ConsumerRepo)
		if sibling == "" {
			continue
		}
		if _, err := os.Stat(sibling); err != nil {
			continue // sibling not checked out; skip
		}
		if err := mustExist(sibling, l.ConsumerPath); err != nil {
			errs = append(errs, fmt.Errorf("consumption_links[%d] consumer_path in sibling %s: %w", i, l.ConsumerRepo, err))
		}
	}
	for i, l := range m.GenerationLinks {
		if l.From != "" {
			if err := mustExist(root, l.From); err != nil {
				errs = append(errs, fmt.Errorf("generation_links[%d].from: %w", i, err))
			}
		}
	}
	for i, g := range m.CIGates {
		if g.Planned {
			continue
		}
		base := root
		if g.Repo != "agent-cluster-contracts" {
			// Skip sibling CI gates entirely in local mode.
			if mode == ModeLocal {
				continue
			}
			base = siblingRoot(root, g.Repo)
			if base == "" {
				continue
			}
			if _, err := os.Stat(base); err != nil {
				continue
			}
		}
		if err := mustExist(base, g.Workflow); err != nil {
			errs = append(errs, fmt.Errorf("ci_gates[%d] workflow: %w", i, err))
		}
	}
	return errs
}

func mustExist(base, rel string) error {
	p := filepath.Join(base, rel)
	if _, err := os.Stat(p); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("path %q does not exist (resolved to %s)", rel, p)
		}
		return err
	}
	return nil
}

// CrossCheck catches semantic drift between the dep map and the concept map
// that path-existence checks cannot see. Specifically: if the dep map's
// `pending` array still mentions constraint C-XXX that the concept map already
// marks as implemented, the pending entry is stale and must be removed.
//
// Decision 022 introduced this check after C-001's cross-repo vocablint
// deployment (D014) shipped, but the dep map kept the "Cross-repo vocab scan
// (constraint C-001 enforcement)" line and two planned:true links for ~8
// decisions. SSOT honesty: when the work is done, the planning artifact must
// say so.
//
// Returns one error per stale mention; empty slice means OK.
func CrossCheck(m *Map, cm *conceptmap.Map) []error {
	if m == nil || cm == nil {
		return nil
	}
	implemented := map[string]bool{}
	for _, c := range cm.Constraints {
		if c.GuardCandidate != nil && c.GuardCandidate.Status == "implemented" {
			implemented[c.ID] = true
		}
	}
	var errs []error
	for i, p := range m.Pending {
		for _, id := range cidRefRe.FindAllString(p, -1) {
			if implemented[id] {
				errs = append(errs, fmt.Errorf("pending[%d] mentions %s which is implemented in concept-map — remove the stale entry. Pending text: %q", i, id, p))
			}
		}
	}
	return errs
}

var cidRefRe = regexp.MustCompile(`C-[0-9]{3}`)

// VerifyFileSystemCoverage enforces that every file under canonical
// SSOT source directories (currently dsl/ and ir/domain/) is registered
// as an ssot_artifact in the dep map. The forward direction is covered
// by `ssotdeps verify` (each declared path must exist on disk); D038
// adds the inverse so newly-added files can't silently bypass the dep
// map's catalog invariant.
//
// Scope is intentionally narrow: directories where each file is a
// canonical SSOT and the set is small/stable. decisions/ is excluded
// (different SSOT pattern — the directory itself is the registry,
// only the foundational initial-agreement is enumerated). tools/ is
// covered by D033 via the workflow-tool coverage check.
//
// root is the contracts repo root. Returns one error per uncovered file.
func VerifyFileSystemCoverage(m *Map, root string) []error {
	if m == nil {
		return nil
	}
	registered := map[string]bool{}
	for _, a := range m.SsotArtifacts {
		registered[filepath.ToSlash(a.Path)] = true
	}
	type rule struct {
		dir    string
		suffix string
	}
	rules := []rule{
		{"dsl", ".lisp"},
		{"ir/domain", ".ir.json"},
	}
	var errs []error
	for _, r := range rules {
		walkErr := filepath.WalkDir(filepath.Join(root, r.dir), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // best-effort; don't fail the cross-check on traversal errors
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), r.suffix) {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return nil
			}
			relSlash := filepath.ToSlash(rel)
			if !registered[relSlash] {
				errs = append(errs, fmt.Errorf(
					"filesystem coverage drift (D038): %s exists on disk but is NOT registered as an ssot_artifact in ssot-dependency-map.riido.json. Add an entry with kind matching its directory (dsl/ → dsl-source, ir/domain/ → ir) so the dep map's catalog invariant is maintained",
					relSlash,
				))
			}
			return nil
		})
		_ = walkErr
	}
	return errs
}

// VerifyToolsReadmeCoverage enforces that every tool in the dep map
// (kind=verifier or kind=code-generator with path starting with
// tools/) is mentioned by name in tools/README.md. Decision 035
// introduced this after audit found docfresh (added by D025) was in
// the dep map but missing from tools/README.md — same drift class as
// D034 (AGENT_CONTRACT.md mirror), applied to the tools catalog.
//
// readmePath is the absolute path to tools/README.md. Returns one
// error per uncovered tool; empty slice means OK.
func VerifyToolsReadmeCoverage(m *Map, readmePath string) []error {
	if m == nil {
		return nil
	}
	data, err := os.ReadFile(readmePath)
	if err != nil {
		return []error{fmt.Errorf("tools/README.md not readable at %s: %w", readmePath, err)}
	}
	readme := string(data)
	var errs []error
	for _, a := range m.SsotArtifacts {
		if a.Kind != "verifier" && a.Kind != "code-generator" {
			continue
		}
		p := filepath.ToSlash(a.Path)
		if !strings.HasPrefix(p, "tools/") || !strings.HasSuffix(p, "/main.go") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(p, "tools/"), "/main.go")
		// Require both the section heading and a path mention so a
		// stray substring in another tool's prose doesn't false-positive.
		hasHeading := strings.Contains(readme, "## "+name+"\n") || strings.Contains(readme, "## "+name+" ")
		hasPath := strings.Contains(readme, a.Path)
		if !hasHeading || !hasPath {
			errs = append(errs, fmt.Errorf(
				"tools/README.md coverage drift (D035): tool %q (ssot_artifact %q, path %s) is missing from tools/README.md. Need a `## %s` section heading AND a reference to `%s` (typically in the 'Where each tool's source lives' table). Found heading=%v, path=%v",
				name, a.ID, a.Path, name, a.Path, hasHeading, hasPath,
			))
		}
	}
	return errs
}

// VerifyAgentContractMirror enforces that AGENT_CONTRACT.md's "Allowed
// paths" and "Forbidden paths" code blocks for the dumb-agent role
// match agent-roles.riido.json's dumb-agent.allowed_paths and
// .forbidden_paths exactly (as sets — order doesn't matter, duplicates
// are not allowed).
//
// Decision 034 introduced this. ssot-dependency-map.riido.json declares
// `agent-roles` with `mirrored_in: ["AGENT_CONTRACT.md"]` — D034 makes
// that mirror claim verifiable. Drift between the JSON SSOT and the
// human-readable mirror is the failure mode D021 and D024 had to ship
// catch-up decisions for; D025 docfresh catches the case where a PR
// adds a rule without doc update, but doesn't catch the case where
// the doc was edited and the JSON wasn't (or vice versa). D034 closes
// that gap for the specific path lists.
//
// agentContractPath is the absolute path to AGENT_CONTRACT.md.
// Returns one error per difference; empty slice means OK.
func VerifyAgentContractMirror(roles *agentguard.Roles, agentContractPath string) []error {
	if roles == nil {
		return nil
	}
	var dumb *agentguard.Role
	for i, r := range roles.Roles {
		if r.ID == "dumb-agent" {
			dumb = &roles.Roles[i]
			break
		}
	}
	if dumb == nil {
		return nil // VerifyForbiddenSymmetry will surface this
	}
	md, err := os.ReadFile(agentContractPath)
	if err != nil {
		return []error{fmt.Errorf("AGENT_CONTRACT.md not readable at %s: %w", agentContractPath, err)}
	}
	allowedMD := extractAgentContractBlock(string(md), "Allowed paths")
	forbiddenMD := extractAgentContractBlock(string(md), "Forbidden paths")

	var errs []error
	errs = append(errs, diffSet("allowed_paths", "dumb-agent.allowed_paths", dumb.AllowedPaths, allowedMD)...)
	errs = append(errs, diffSet("forbidden_paths", "dumb-agent.forbidden_paths", dumb.ForbiddenPaths, forbiddenMD)...)
	return errs
}

// extractAgentContractBlock returns the lines of the first fenced
// code block (```...```) that follows a "### <heading>" line in md.
// Returns nil if no such block is found.
func extractAgentContractBlock(md, heading string) []string {
	pat := regexp.MustCompile(`### ` + regexp.QuoteMeta(heading) + `[^\n]*\n+` + "```" + `\n([\s\S]*?)\n` + "```")
	m := pat.FindStringSubmatch(md)
	if m == nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		t := strings.TrimSpace(line)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// diffSet compares two lists as sets, returning one error per
// element that's in one list but not the other.
func diffSet(label, source string, fromJSON, fromMD []string) []error {
	jset := map[string]bool{}
	for _, s := range fromJSON {
		jset[s] = true
	}
	mset := map[string]bool{}
	for _, s := range fromMD {
		mset[s] = true
	}
	var errs []error
	for s := range jset {
		if !mset[s] {
			errs = append(errs, fmt.Errorf("AGENT_CONTRACT.md mirror drift (D034): %q is in %s but missing from the AGENT_CONTRACT.md `### %s` code block. Add it to the markdown or remove it from the JSON", s, source, label))
		}
	}
	for s := range mset {
		if !jset[s] {
			errs = append(errs, fmt.Errorf("AGENT_CONTRACT.md mirror drift (D034): %q is in the AGENT_CONTRACT.md `### %s` code block but missing from %s. Add it to the JSON or remove it from the markdown", s, label, source))
		}
	}
	return errs
}

// VerifyWorkflowToolCoverage enforces that every tool referenced from a
// GitHub Actions workflow as `./bin/<name>` is registered as an
// ssot_artifact in the dep map. Decision 033 introduced this after an
// audit found 6 CI-bound tools (agentguard, conceptmap, decision,
// irdrift, secretscan, ssotdeps) on disk and in workflow run steps but
// missing from the dep map's catalog — invisible to D031/D032 audits
// and to the dep map's "what tools belong to contracts" invariant.
//
// workflowsDir is typically root/.github/workflows. Returns one error
// per uncovered ./bin/<name> reference; empty slice means OK.
func VerifyWorkflowToolCoverage(m *Map, workflowsDir string) []error {
	if m == nil {
		return nil
	}
	registered := map[string]bool{}
	for _, a := range m.SsotArtifacts {
		if strings.HasPrefix(filepath.ToSlash(a.Path), "tools/") && strings.HasSuffix(a.Path, "/main.go") {
			name := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(a.Path), "tools/"), "/main.go")
			registered[name] = true
		}
	}
	entries, err := os.ReadDir(workflowsDir)
	if err != nil {
		return nil // no workflows dir to check; not an error in this context
	}
	// The preceding character must NOT be `.` so we don't match `../bin/x`
	// as `./bin/x`. The capture group is the tool name.
	binRe := regexp.MustCompile(`(?:^|[^.])\./bin/([a-z][a-z0-9-]*)\b`)
	type ref struct {
		tool     string
		workflow string
	}
	var refs []ref
	seen := map[ref]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(workflowsDir, e.Name()))
		if err != nil {
			continue
		}
		for _, match := range binRe.FindAllStringSubmatch(string(data), -1) {
			r := ref{tool: match[1], workflow: e.Name()}
			if !seen[r] {
				seen[r] = true
				refs = append(refs, r)
			}
		}
	}
	var errs []error
	for _, r := range refs {
		if !registered[r.tool] {
			errs = append(errs, fmt.Errorf(
				"workflow %s references ./bin/%s but tools/%s/main.go is not registered as an ssot_artifact in ssot-dependency-map.riido.json (D033). Add `{ \"id\": \"%s-tool\", \"kind\": \"verifier\", \"path\": \"tools/%s/main.go\", \"schema\": null, \"owned_by\": \"agent-cluster-contracts\" }`",
				r.workflow, r.tool, r.tool, r.tool, r.tool,
			))
		}
	}
	return errs
}

// VerifyForbiddenSymmetry enforces C-016 (D031) executably: for every
// SSOT artifact in the dep map that declares both a data path and a
// non-empty schema path, BOTH must be covered by the dumb-agent role's
// forbidden_paths globs in agent-roles.riido.json. Asymmetry — one
// half covered, the other implicit — is rejected.
//
// Decision 032 introduced this check. The C-016 rule was prose only
// until D032; the next time someone added an SSOT and forgot the
// schema (or vice versa), nothing automatic would have caught it.
//
// Returns one error per asymmetric pair; empty slice means OK.
func VerifyForbiddenSymmetry(m *Map, roles *agentguard.Roles) []error {
	if m == nil || roles == nil {
		return nil
	}
	var dumb *agentguard.Role
	for i, r := range roles.Roles {
		if r.ID == "dumb-agent" {
			dumb = &roles.Roles[i]
			_ = r
			break
		}
	}
	if dumb == nil {
		return []error{fmt.Errorf("agent-roles: no role with id=dumb-agent (cannot enforce C-016)")}
	}
	var errs []error
	for _, a := range m.SsotArtifacts {
		if a.Schema == nil || *a.Schema == "" {
			continue
		}
		dataPat, dataCovered := agentguard.MatchAny(a.Path, dumb.ForbiddenPaths)
		schemaPat, schemaCovered := agentguard.MatchAny(*a.Schema, dumb.ForbiddenPaths)
		if dataCovered != schemaCovered {
			covered := "data"
			missing := "schema"
			missingPath := *a.Schema
			pat := dataPat
			if schemaCovered {
				covered, missing = "schema", "data"
				missingPath = a.Path
				pat = schemaPat
			}
			errs = append(errs, fmt.Errorf(
				"C-016 asymmetry: ssot_artifact %q has its %s covered by dumb-agent forbidden_paths pattern %q but the %s file %q is NOT covered. Either add an explicit entry for %s in agent-roles.riido.json, or add a broader glob, so both halves are guarded together",
				a.ID, covered, pat, missing, missingPath, missingPath,
			))
		}
	}
	return errs
}

// siblingRoot maps "agent-cluster-backend" → "<parent-of-root>/backend" and
// "agent-cluster-frontend" → "<parent-of-root>/frontend". The local checkout
// uses short dir names per the bootstrap convention.
func siblingRoot(contractsRoot, repo string) string {
	parent := filepath.Dir(contractsRoot)
	switch repo {
	case "agent-cluster-backend":
		return filepath.Join(parent, "backend")
	case "agent-cluster-frontend":
		return filepath.Join(parent, "frontend")
	case "agent-cluster-contracts":
		return contractsRoot
	}
	return ""
}
