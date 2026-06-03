// Package probe is the dumb-agent integration entry point. It answers two
// questions:
//
//  1. Is the baseline green right now? (Preflight)
//     Runs every contracts-local verifier and returns a structured result
//     with status candidate_allowed or baseline_blocked. The dumb-agent must
//     refuse to create any domain candidates while the baseline is red.
//
//  2. Are the existing fixtures classified correctly? (Fixtures)
//     Walks fixtures/positive/** and fixtures/negative/**. Each .json fixture
//     is paired with a .meta.json declaring its fixture_type and expected
//     outcome; the verifier runs the matching schema check and confirms the
//     real outcome equals the expected one.
//
// First version intentionally supports only decision fixtures; other types
// require a separate decision before they are added.
package probe

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/kimjooyoon/agent-cluster-contracts/internal/agentguard"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/codegen"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/conceptmap"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/decision"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/irdrift"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/jsonutil"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/secretscan"
	"github.com/kimjooyoon/agent-cluster-contracts/internal/ssotdeps"
)

// PreflightStatus is the machine-readable status returned to the dumb-agent.
type PreflightStatus string

const (
	StatusCandidateAllowed PreflightStatus = "candidate_allowed"
	StatusBaselineBlocked  PreflightStatus = "baseline_blocked"
)

// CheckResult is one baseline check's outcome.
type CheckResult struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

// PreflightResult is what `probe preflight` emits as JSON.
type PreflightResult struct {
	OK                bool            `json:"ok"`
	Status            PreflightStatus `json:"status"`
	Blockers          []CheckResult   `json:"blockers"`
	AllChecks         []CheckResult   `json:"all_checks"`
	CandidateAllowed  bool            `json:"candidate_allowed"`
	NextAllowedAction string          `json:"next_allowed_action"`
}

// Preflight runs every contracts-local baseline check. It NEVER reads
// sibling-repo state — ssotdeps runs in ModeLocal. The dumb-agent reads the
// resulting Status and obeys it without further reasoning.
func Preflight(root string) PreflightResult {
	var checks []CheckResult

	checks = append(checks, runDecisions(root))
	checks = append(checks, runSsotdeps(root))
	checks = append(checks, runSsotdepsValidate(root))
	checks = append(checks, runConceptmap(root))
	checks = append(checks, runAgentRoles(root))
	checks = append(checks, runSecretscan(root))
	checks = append(checks, runIRDrift(root))

	res := PreflightResult{AllChecks: checks}
	for _, c := range checks {
		if !c.OK {
			res.Blockers = append(res.Blockers, c)
		}
	}
	if len(res.Blockers) == 0 {
		res.OK = true
		res.Status = StatusCandidateAllowed
		res.CandidateAllowed = true
		res.NextAllowedAction = "create-candidate"
	} else {
		res.OK = false
		res.Status = StatusBaselineBlocked
		res.CandidateAllowed = false
		res.NextAllowedAction = "stop"
	}
	return res
}

func runDecisions(root string) CheckResult {
	records, err := decision.LoadAll(root)
	if err != nil {
		return CheckResult{Name: "decision-validate", OK: false, Summary: "load: " + err.Error()}
	}
	var failing int
	var detail strings.Builder
	for _, r := range records {
		errs := decision.Validate(r)
		if len(errs) > 0 {
			failing++
			fmt.Fprintf(&detail, "%s: %d error(s)\n", r.ID, len(errs))
		}
	}
	if failing > 0 {
		return CheckResult{
			Name: "decision-validate", OK: false,
			Summary: fmt.Sprintf("%d/%d records invalid", failing, len(records)),
			Detail:  detail.String(),
		}
	}
	return CheckResult{
		Name: "decision-validate", OK: true,
		Summary: fmt.Sprintf("%d records valid", len(records)),
	}
}

func runSsotdeps(root string) CheckResult {
	m, err := ssotdeps.Load(root)
	if err != nil {
		return CheckResult{Name: "ssotdeps-local", OK: false, Summary: "load: " + err.Error()}
	}
	errs := ssotdeps.Verify(root, m, ssotdeps.ModeLocal)
	if len(errs) > 0 {
		return CheckResult{
			Name: "ssotdeps-local", OK: false,
			Summary: fmt.Sprintf("%d error(s) (mode=local)", len(errs)),
			Detail:  joinErrors(errs),
		}
	}
	return CheckResult{
		Name: "ssotdeps-local", OK: true,
		Summary: fmt.Sprintf("OK (%d artifacts, %d links, %d gates, mode=local)",
			len(m.SsotArtifacts), len(m.ConsumptionLinks), len(m.CIGates)),
	}
}

func runSsotdepsValidate(root string) CheckResult {
	m, err := ssotdeps.Load(root)
	if err != nil {
		return CheckResult{Name: "ssot-dep-map-validate", OK: false, Summary: "load: " + err.Error()}
	}
	errs := ssotdeps.ValidateMap(m)
	if len(errs) > 0 {
		return CheckResult{
			Name: "ssot-dep-map-validate", OK: false,
			Summary: fmt.Sprintf("%d error(s)", len(errs)),
			Detail:  joinErrors(errs),
		}
	}
	return CheckResult{
		Name: "ssot-dep-map-validate", OK: true,
		Summary: fmt.Sprintf("OK (%d artifacts)", len(m.SsotArtifacts)),
	}
}

func runConceptmap(root string) CheckResult {
	m, err := conceptmap.Load(root)
	if err != nil {
		return CheckResult{Name: "conceptmap-verify", OK: false, Summary: "load: " + err.Error()}
	}
	errs := conceptmap.Validate(m)
	if len(errs) > 0 {
		return CheckResult{
			Name: "conceptmap-verify", OK: false,
			Summary: fmt.Sprintf("%d error(s)", len(errs)),
			Detail:  joinErrors(errs),
		}
	}
	return CheckResult{
		Name: "conceptmap-verify", OK: true,
		Summary: fmt.Sprintf("OK (%d concepts, %d relationships, %d constraints)",
			len(m.Concepts), len(m.Relationships), len(m.Constraints)),
	}
}

func runAgentRoles(root string) CheckResult {
	roles, err := agentguard.Load(root)
	if err != nil {
		return CheckResult{Name: "agent-roles-validate", OK: false, Summary: "load: " + err.Error()}
	}
	errs := agentguard.ValidateRoles(roles)
	if len(errs) > 0 {
		return CheckResult{
			Name: "agent-roles-validate", OK: false,
			Summary: fmt.Sprintf("%d error(s)", len(errs)),
			Detail:  joinErrors(errs),
		}
	}
	return CheckResult{
		Name: "agent-roles-validate", OK: true,
		Summary: fmt.Sprintf("OK (%d role(s))", len(roles.Roles)),
	}
}

func runSecretscan(root string) CheckResult {
	findings, err := secretscan.Scan(root, secretscan.DefaultPatterns(), secretscan.DefaultSkip())
	if err != nil {
		return CheckResult{Name: "secretscan", OK: false, Summary: "scan: " + err.Error()}
	}
	if len(findings) > 0 {
		var detail strings.Builder
		for _, f := range findings {
			fmt.Fprintf(&detail, "%s:%d: %s\n", f.Path, f.Line, f.Pattern)
		}
		return CheckResult{
			Name: "secretscan", OK: false,
			Summary: fmt.Sprintf("%d finding(s)", len(findings)),
			Detail:  detail.String(),
		}
	}
	return CheckResult{Name: "secretscan", OK: true, Summary: "no findings"}
}

func runIRDrift(root string) CheckResult {
	res, err := irdrift.Check(root)
	if err != nil {
		// SBCL missing, emitter exploded, etc — true environment blocker.
		return CheckResult{
			Name: "irdrift", OK: false,
			Summary: "check failed: " + err.Error(),
			Detail:  "irdrift requires sbcl on PATH and writable temp dir",
		}
	}
	drift := len(res.Differs) + len(res.MissingInTemp) + len(res.ExtraInTemp)
	if drift > 0 {
		return CheckResult{
			Name: "irdrift", OK: false,
			Summary: fmt.Sprintf("%d drift(s)", drift),
		}
	}
	return CheckResult{Name: "irdrift", OK: true, Summary: "IR matches DSL"}
}

func joinErrors(errs []error) string {
	var b strings.Builder
	for _, e := range errs {
		b.WriteString(e.Error())
		b.WriteByte('\n')
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Fixture verifier
// ---------------------------------------------------------------------------

// FixtureMeta is the schema of <name>.meta.json next to each fixture.
type FixtureMeta struct {
	// FixtureType identifies which validator to run.
	FixtureType string `json:"fixture_type"`
	// Expected is "pass" or "fail".
	Expected string `json:"expected"`
	// ExpectedErrorCategory is informational; v1 just matches Contains.
	ExpectedErrorCategory string `json:"expected_error_category,omitempty"`
	// ExpectedErrorContains: when set, the actual error string must contain
	// this substring. Only meaningful when Expected = "fail".
	ExpectedErrorContains string `json:"expected_error_contains,omitempty"`
	// FromRole records which role created the fixture (for audit).
	FromRole string `json:"from_role,omitempty"`
	// Purpose declares what this fixture proves. Decision 015 added this as
	// required. Two fixtures in the same (category, fixture_type) cannot
	// share the same purpose — that's the dedup rule that catches structurally
	// identical noise (e.g. cycle-N fixtures that only differ by id/title).
	Purpose string `json:"purpose"`
}

// FixtureCheck is one fixture's outcome.
type FixtureCheck struct {
	Path     string `json:"path"`
	OK       bool   `json:"ok"`
	Category string `json:"category"` // positive | negative | fuzz | candidate
	Reason   string `json:"reason"`
}

// purposeBanlist is the decoded purpose-banlist.riido.json. Decision 018.
type purposeBanlist struct {
	Banned []struct {
		Normalized        string `json:"normalized"`
		SeededByDecision  string `json:"seeded_by_decision"`
		Reason            string `json:"reason"`
	} `json:"banned"`
}

// loadPurposeBanlist reads root/purpose-banlist.riido.json. Returns empty
// list (not error) when the file is absent — keeps probe fixtures usable in
// fresh-clone test trees that don't have the banlist yet.
func loadPurposeBanlist(root string) purposeBanlist {
	var bl purposeBanlist
	path := filepath.Join(root, "purpose-banlist.riido.json")
	if _, err := os.Stat(path); err != nil {
		return bl
	}
	_ = jsonutil.ReadFile(path, &bl)
	return bl
}

// banlistMatch returns the seeded_by_decision and reason for the first
// banlist entry whose normalized form equals normalizedPurpose. Empty
// strings if no match.
func banlistMatch(bl purposeBanlist, normalizedPurpose string) (decision, reason string) {
	for _, b := range bl.Banned {
		if b.Normalized == normalizedPurpose {
			return b.SeededByDecision, b.Reason
		}
	}
	return "", ""
}

// purposeNormalizationStrip removes mechanical noise that an agent might
// append to make duplicate purposes look unique. Decision 017 introduced
// this. Stripped patterns:
//
//   - parenthesized runs of digits (Unix timestamps): "foo (1780485421)" → "foo"
//   - "cycle N" / "cycle-N" tokens: "Foo cycle 14" → "Foo"
//   - trailing/leading whitespace and multiple internal spaces collapsed
//   - lowercased for case-insensitive comparison
//
// Adding a new pattern requires a decision record (so the rule can't be
// silently weakened by future commits).
var (
	purposeStripParenNumeric = regexp.MustCompile(`\(\s*\d+\s*\)`)
	purposeStripCycleTokens  = regexp.MustCompile(`(?i)\bcycle[\s-]*\d+\b`)
	purposeStripBareNumeric  = regexp.MustCompile(`\b\d{6,}\b`) // long bare numbers (timestamps without parens)
	purposeCollapseSpaces    = regexp.MustCompile(`\s+`)
)

// noiseMarkerReason returns a non-empty rejection reason if meta, the
// fixture filename, or the fixture data carry the dumb-agent's
// generator fingerprint:
//
//   - meta.Purpose (RAW, before normalize) contains a `cycle-N` token (D023).
//   - The fixture file PATH contains a `cycle-N` token (D026) — catches the
//     `decision-positive-cycle-EPOCH.json` filename pattern even when the
//     purpose and id have been scrubbed.
//   - The fixture data file's RAW content contains a `cycle-N` token (D026)
//     — catches cycle-N leftover in title/examples/evidence.ref when the
//     dumb-agent forgets to scrub one of them.
//   - The fixture meta file's RAW content contains a `cycle-N` token (D026)
//     — defensive scan over the full meta JSON, not just purpose.
//   - For decision fixtures, the data file's `id` does NOT match
//     `^000-fixture-` (D026, supersedes D023's narrower 999- check). The
//     000-fixture-* range is reserved for canonical/designer-authored
//     fixtures; any other prefix is rejected because it's the dumb-agent
//     trying to look like a real decision (observed: 102-, 103-).
//
// The signal is *structural* (the generator's tells), not *semantic* (what
// the purpose words say), so the dumb-agent can't dodge it by rephrasing
// alone — it would have to change its generator code (which lives outside
// its allowed paths).
func noiseMarkerReason(meta FixtureMeta, dataPath string) string {
	if cycleMarkerRe.MatchString(meta.Purpose) {
		return fmt.Sprintf(
			"noise marker (D023): purpose %q contains a `cycle-N` token. Purposes must describe the rule under test, not the iteration that produced the fixture",
			meta.Purpose,
		)
	}
	if cycleMarkerRe.MatchString(filepath.Base(dataPath)) {
		return fmt.Sprintf(
			"noise marker (D026): fixture filename %q contains a `cycle-N` token. Filenames must describe what the fixture tests, not the iteration that produced it",
			filepath.Base(dataPath),
		)
	}
	if dataBytes, err := os.ReadFile(dataPath); err == nil {
		if loc := cycleMarkerRe.FindIndex(dataBytes); loc != nil {
			return fmt.Sprintf(
				"noise marker (D026): fixture data contains a `cycle-N` token (%q at byte %d). Scrub the cycle-iteration marker from id, title, examples, and evidence.ref",
				string(dataBytes[loc[0]:loc[1]]), loc[0],
			)
		}
	}
	metaPath := strings.TrimSuffix(dataPath, ".json") + ".meta.json"
	if metaBytes, err := os.ReadFile(metaPath); err == nil {
		if loc := cycleMarkerRe.FindIndex(metaBytes); loc != nil {
			return fmt.Sprintf(
				"noise marker (D026): fixture meta contains a `cycle-N` token (%q at byte %d) outside the purpose field",
				string(metaBytes[loc[0]:loc[1]]), loc[0],
			)
		}
	}
	// D028: case-insensitive substring scan for "dumb-agent" in the
	// fixture data file. The dumb-agent's template generator emits
	// title "Positive fixture generated by dumb-agent cycle" — even
	// after scrubbing cycle-N from filename, purpose, id, and examples,
	// it left this literal substring as the smoking gun. Reject any
	// fixture data that names the generator by name. Designer-authored
	// fixtures describe the rule under test, not the actor that produced
	// the file.
	if dataBytes, err := os.ReadFile(dataPath); err == nil {
		if loc := dumbAgentMarkerRe.FindIndex(dataBytes); loc != nil {
			return fmt.Sprintf(
				"noise marker (D028): fixture data contains the literal `dumb-agent` substring (%q at byte %d). Fixture content must describe what the schema rule under test is, not which actor produced the file",
				string(dataBytes[loc[0]:loc[1]]), loc[0],
			)
		}
	}
	// The id rule applies to fixtures whose PATH places them under
	// fixtures/<category>/decision/. We anchor on path rather than
	// meta.FixtureType because some legacy fixtures (under work-item/)
	// declare fixture_type=decision even though their data is IR —
	// that mismatch is a separate backlog item; D026 should not
	// double-fire on it.
	if isDecisionFixturePath(dataPath) {
		var data struct {
			ID string `json:"id"`
		}
		if err := jsonutil.ReadFile(dataPath, &data); err == nil {
			if !decisionFixtureIDRe.MatchString(data.ID) {
				return fmt.Sprintf(
					"noise marker (D026): decision fixture id %q does not match the reserved canonical pattern `^000-fixture-`. Fixtures must live in the 000-fixture- namespace (000-fixture-positive-minimal, 000-fixture-negative-missing-owner, …). Real decision records under decisions/ keep using sequential NNN-slug; only fixture data files are restricted",
					data.ID,
				)
			}
		}
	}
	return ""
}

func isDecisionFixturePath(p string) bool {
	s := filepath.ToSlash(p)
	return strings.Contains(s, "/fixtures/positive/decision/") || strings.Contains(s, "/fixtures/negative/decision/")
}

var (
	cycleMarkerRe       = regexp.MustCompile(`(?i)\bcycle[\s\-]\d+\b`)
	decisionFixtureIDRe = regexp.MustCompile(`^000-fixture-`)
	dumbAgentMarkerRe   = regexp.MustCompile(`(?i)dumb-agent`)
)

// NormalizePurpose returns the comparable form used by D015 dedup after D017.
// Two purposes are duplicates if their NormalizePurpose results are equal.
func NormalizePurpose(p string) string {
	s := p
	s = purposeStripParenNumeric.ReplaceAllString(s, "")
	s = purposeStripCycleTokens.ReplaceAllString(s, "")
	s = purposeStripBareNumeric.ReplaceAllString(s, "")
	s = purposeCollapseSpaces.ReplaceAllString(s, " ")
	s = strings.ToLower(strings.TrimSpace(s))
	return s
}

// FixturesResult is the aggregate output of `probe fixtures`.
type FixturesResult struct {
	OK      bool           `json:"ok"`
	Checks  []FixtureCheck `json:"checks"`
	Skipped int            `json:"skipped"`
}

// VerifyFixtures walks contracts/fixtures/ and validates each .json fixture
// according to its .meta.json. Returns an OK FixturesResult when there are no
// fixtures (e.g. fresh repo), so empty trees do not block the baseline.
func VerifyFixtures(root string) (FixturesResult, error) {
	res := FixturesResult{OK: true}
	fixturesDir := filepath.Join(root, "fixtures")
	if _, err := os.Stat(fixturesDir); errors.Is(err, fs.ErrNotExist) {
		return res, nil
	}

	type pair struct {
		fixturePath string
		category    string
	}
	var pairs []pair
	err := filepath.WalkDir(fixturesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") || strings.HasSuffix(d.Name(), ".meta.json") {
			return nil
		}
		// Classify by parent directory under fixtures/
		rel, _ := filepath.Rel(fixturesDir, path)
		category := "unknown"
		if strings.HasPrefix(filepath.ToSlash(rel), "positive/") {
			category = "positive"
		} else if strings.HasPrefix(filepath.ToSlash(rel), "negative/") {
			category = "negative"
		}
		pairs = append(pairs, pair{path, category})
		return nil
	})
	if err != nil {
		return res, err
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].fixturePath < pairs[j].fixturePath })

	// dedup tracks (category, fixture_type, NORMALIZED purpose) → first path
	// that claimed it. Decision 015 introduced this; decision 017 added the
	// normalization so timestamp/cycle-suffix gaming gets caught too —
	// "Foo (1780485421)" and "Foo (1780485142)" both normalize to "foo" and
	// the second is rejected.
	type purposeKey struct {
		category    string
		fixtureType string
		purpose     string
	}
	seenPurposes := map[purposeKey]string{}

	// Decision 018: persistent banlist of known noise templates.
	banlist := loadPurposeBanlist(root)

	for _, p := range pairs {
		check := verifyOneFixture(p.fixturePath, p.category)
		// On top of the per-fixture check, enforce purpose-based dedup.
		// We need to re-read the meta to access Purpose; verifyOneFixture
		// already did this but didn't surface it. Reload (cheap; fixtures
		// are tiny).
		metaPath := strings.TrimSuffix(p.fixturePath, ".json") + ".meta.json"
		var meta FixtureMeta
		_ = jsonutil.ReadFile(metaPath, &meta)
		if meta.FixtureType != "" && check.OK {
			// D023: structural noise-marker check. Banlist (D018) catches only
			// templates that have already shown up at least once; this catches
			// the dumb-agent's generator fingerprint on first occurrence so
			// each new wording variant doesn't need its own decision record.
			if reason := noiseMarkerReason(meta, p.fixturePath); reason != "" {
				check.OK = false
				check.Reason = reason
				if !check.OK {
					res.OK = false
				}
				res.Checks = append(res.Checks, check)
				continue
			}
			normalized := NormalizePurpose(meta.Purpose)
			// D018 banlist check — happens before dedup so banned templates
			// fail even on first occurrence in this set.
			if decision, reason := banlistMatch(banlist, normalized); decision != "" {
				check.OK = false
				check.Reason = fmt.Sprintf(
					"banned purpose template (normalized: %q) — added by decision %s; reason: %s",
					normalized, decision, reason,
				)
			} else {
				key := purposeKey{
					category:    p.category,
					fixtureType: meta.FixtureType,
					purpose:     normalized,
				}
				if firstPath, dup := seenPurposes[key]; dup {
					check.OK = false
					check.Reason = fmt.Sprintf(
						"duplicate purpose (normalized: %q): same (category=%s, fixture_type=%s) already claimed by %s — declare a meaningfully novel purpose, not a timestamp- or cycle-suffixed variant",
						normalized, p.category, meta.FixtureType, firstPath,
					)
				} else {
					seenPurposes[key] = p.fixturePath
				}
			}
		}
		if !check.OK {
			res.OK = false
		}
		res.Checks = append(res.Checks, check)
	}
	return res, nil
}

func verifyOneFixture(path, category string) FixtureCheck {
	metaPath := strings.TrimSuffix(path, ".json") + ".meta.json"
	meta := FixtureMeta{}
	if err := jsonutil.ReadFile(metaPath, &meta); err != nil {
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "missing or invalid meta sidecar (expected " + filepath.Base(metaPath) + "): " + err.Error(),
		}
	}
	if meta.Expected != "pass" && meta.Expected != "fail" {
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "meta.expected must be \"pass\" or \"fail\", got " + meta.Expected,
		}
	}
	if category == "negative" && meta.Expected != "fail" {
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "negative fixture must declare meta.expected=fail",
		}
	}
	if category == "positive" && meta.Expected != "pass" {
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "positive fixture must declare meta.expected=pass",
		}
	}
	if strings.TrimSpace(meta.Purpose) == "" {
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "meta.purpose required (decision 015): declare what this fixture proves",
		}
	}

	switch meta.FixtureType {
	case "decision":
		return runDecisionFixture(path, category, meta)
	case "ir-aggregate":
		return runIRFixture(path, category, meta, "aggregate")
	case "ir-event":
		return runIRFixture(path, category, meta, "event")
	case "query":
		return runIRFixture(path, category, meta, "query")
	case "":
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "meta.fixture_type required (supported: decision, ir-aggregate, ir-event, query)",
		}
	default:
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "meta.fixture_type " + meta.FixtureType + " not supported; supported: decision, ir-aggregate, ir-event, query",
		}
	}
}

func runDecisionFixture(path, category string, meta FixtureMeta) FixtureCheck {
	r := &decision.Record{}
	if err := jsonutil.ReadFile(path, r); err != nil {
		// Parse failure: counts as a validation failure for the fixture.
		actualErr := err.Error()
		return judgeOutcome(path, category, meta, false, actualErr)
	}
	errs := decision.Validate(r)
	if len(errs) == 0 {
		return judgeOutcome(path, category, meta, true, "")
	}
	var msgs []string
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return judgeOutcome(path, category, meta, false, strings.Join(msgs, "; "))
}

// runIRFixture validates an IR fixture against the structural rules defined
// by ir/schema/ir.schema.json (mirrored in code to avoid a third-party JSON
// Schema validator dependency). expectedKind is the kind the meta type
// promises (aggregate / event / query).
func runIRFixture(path, category string, meta FixtureMeta, expectedKind string) FixtureCheck {
	doc := &codegen.IRDoc{}
	if err := jsonutil.ReadFile(path, doc); err != nil {
		return judgeOutcome(path, category, meta, false, "parse: "+err.Error())
	}
	errs := validateIRDoc(doc, expectedKind)
	if len(errs) == 0 {
		return judgeOutcome(path, category, meta, true, "")
	}
	var msgs []string
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	return judgeOutcome(path, category, meta, false, strings.Join(msgs, "; "))
}

var (
	irKebabRe  = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	irSHARe    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	irDSLPath  = regexp.MustCompile(`^dsl/.+\.lisp$`)
	irKindEnum = map[string]bool{"aggregate": true, "event": true, "query": true}
	irShape    = map[string]bool{"list": true, "one": true}
)

// validateIRDoc mirrors ir/schema/ir.schema.json in code. Returns one error per
// rule violation. If the document is well-formed but its kind disagrees with
// the meta's promise, that's also a violation (meta lied about fixture_type).
func validateIRDoc(d *codegen.IRDoc, expectedKind string) []error {
	var errs []error
	if !irKindEnum[d.Kind] {
		errs = append(errs, fmt.Errorf("kind %q: must be one of aggregate|event|query", d.Kind))
	}
	if d.Kind != expectedKind {
		errs = append(errs, fmt.Errorf("kind %q does not match meta fixture_type promise %q", d.Kind, irFixtureTypeFor(expectedKind)))
	}
	if !irKebabRe.MatchString(d.Name) {
		errs = append(errs, fmt.Errorf("name %q: must be kebab-case (^[a-z][a-z0-9-]*$)", d.Name))
	}
	if !irDSLPath.MatchString(d.Source.DSLFile) {
		errs = append(errs, fmt.Errorf("source.dsl_file %q: must match dsl/*.lisp", d.Source.DSLFile))
	}
	if !irSHARe.MatchString(d.Source.SHA256) {
		errs = append(errs, fmt.Errorf("source.sha256 %q: must be 64 hex chars", d.Source.SHA256))
	}
	switch d.Kind {
	case "aggregate", "event":
		if len(d.Slots) == 0 {
			errs = append(errs, fmt.Errorf("kind=%s requires at least one slot", d.Kind))
		}
		if d.WireName != "" || d.Returns != nil {
			errs = append(errs, fmt.Errorf("kind=%s must not declare wire_name or returns", d.Kind))
		}
		for i, s := range d.Slots {
			if !irKebabRe.MatchString(s.Name) {
				errs = append(errs, fmt.Errorf("slots[%d].name %q: must be kebab-case", i, s.Name))
			}
			if s.Type == "" {
				errs = append(errs, fmt.Errorf("slots[%d].type: required", i))
			}
		}
	case "query":
		if len(d.Slots) != 0 {
			errs = append(errs, fmt.Errorf("kind=query must not declare slots"))
		}
		if d.WireName == "" {
			errs = append(errs, fmt.Errorf("kind=query requires wire_name"))
		}
		if d.Returns == nil {
			errs = append(errs, fmt.Errorf("kind=query requires returns"))
		} else {
			if !irShape[d.Returns.Shape] {
				errs = append(errs, fmt.Errorf("returns.shape %q: must be list|one", d.Returns.Shape))
			}
			if !irKebabRe.MatchString(d.Returns.Type) {
				errs = append(errs, fmt.Errorf("returns.type %q: must be kebab-case", d.Returns.Type))
			}
		}
	}
	return errs
}

func irFixtureTypeFor(kind string) string {
	switch kind {
	case "aggregate":
		return "ir-aggregate"
	case "event":
		return "ir-event"
	case "query":
		return "query"
	}
	return kind
}

func judgeOutcome(path, category string, meta FixtureMeta, passed bool, actualErr string) FixtureCheck {
	if passed && meta.Expected == "pass" {
		return FixtureCheck{Path: path, Category: category, OK: true, Reason: "validated as expected"}
	}
	if !passed && meta.Expected == "fail" {
		if meta.ExpectedErrorContains != "" && !strings.Contains(actualErr, meta.ExpectedErrorContains) {
			return FixtureCheck{
				Path: path, Category: category, OK: false,
				Reason: fmt.Sprintf("failed (as expected) but error %q does not contain expected substring %q",
					actualErr, meta.ExpectedErrorContains),
			}
		}
		return FixtureCheck{Path: path, Category: category, OK: true, Reason: "failed as expected"}
	}
	if passed && meta.Expected == "fail" {
		return FixtureCheck{
			Path: path, Category: category, OK: false,
			Reason: "expected fixture to fail but it validated; write a guard-candidate note describing the gap",
		}
	}
	return FixtureCheck{
		Path: path, Category: category, OK: false,
		Reason: fmt.Sprintf("expected pass but failed: %s", actualErr),
	}
}
