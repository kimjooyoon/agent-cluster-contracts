# Agent Contract

**The first file every agent reads.** Mirror of [`agent-roles.riido.json`](agent-roles.riido.json) +
[`initial-agreement.md`](initial-agreement.md). If this file and the JSON SSOT
disagree, the JSON wins.

There are multiple agent classes in this project, and each is bounded by an
explicit role with allowed and forbidden paths. The boundaries are enforced by
`tools/agentguard` (Go binary) and the `dumb-agent-safety.yml` workflow.

---

## Roles

| Role           | Purpose                                                    | Bounded by          |
|----------------|------------------------------------------------------------|---------------------|
| `designer`     | Primary author — humans and high-context agents like Claude/Codex | manual review + all other CI gates |
| `dumb-agent`   | Contract Fuzzer / IR Mutation Scout (this is *you* if you are weak) | agentguard + dumb-agent-safety.yml |
| `guard-author` | Modifies verifier code, schemas, tests                     | agentguard (forbids candidate-space mixing) |

To run as a non-designer role, your PR must signal the role explicitly:

- **Label** the PR with the role name: `dumb-agent` or `guard-author`.
- Or use a branch prefix: `dumb-agent/<topic>` or `guard-author/<topic>`.

CI looks for either signal. Without one, the PR is treated as `designer` and
goes through normal review.

## Preflight checks (run these BEFORE you open a PR)

Decisions 004 + 005 added structured diagnostics and a baseline probe so
weak agents can self-correct and never operate under a broken baseline. Run
these locally before pushing — CI runs the same checks but failing in CI is
a slower feedback loop than failing on your laptop.

### Step 0 — build the tools

```sh
go build -o bin/ ./tools/...
```

### Step 1 — read your role

```sh
./bin/agentguard show --role dumb-agent
```

Outputs: allowed_paths, forbidden_paths, max_files_per_pr, auto_merge_paths,
PR-isolation rules, the decision that introduced the role. **If your planned
PR doesn't fit these rules, do not even start writing files.**

### Step 2 — probe the baseline

```sh
./bin/probe preflight --json
```

This runs every contracts-local verifier (`decision validate`, `ssotdeps
verify --mode local`, `conceptmap verify`, `secretscan`, `irdrift`) and
returns one of:

> Note: contracts CI also runs `ssotdeps cross-check` (D022), which
> fails if the dep map's `pending[]` array still mentions a constraint
> id that concept-map already marks as `implemented`. You don't need
> to run this locally — it's enforced at PR time — but if you touch
> `ssot-dependency-map.riido.json` and want to be sure, run
> `./bin/ssotdeps cross-check`.

> Note: contracts CI also runs `docfresh check` (D025) on PR events.
> If your PR adds a decision record whose `guards[].ref` points at a
> dumb-agent-visible rule (anything under `internal/probe`,
> `internal/agentguard`, `purpose-banlist`, or `agent-roles`), the
> diff must also touch this file (`AGENT_CONTRACT.md`). A one-line
> acknowledgement is enough — the goal is just to make sure the doc
> never lags behind the executable rules dumb-agent reads first.


- `"status": "candidate_allowed"` → baseline is green. You may create
  **exactly one small candidate** (≤ 5 files; see role config). Proceed to
  Step 3.
- `"status": "baseline_blocked"` → baseline is RED. **STOP.** Do not write
  fixtures, fuzz corpus, IR candidates, or domain guard-candidate notes
  under a red baseline. You may file **at most one**
  `reports/environment-blockers/<id>-<slug>.md` describing what
  `probe preflight` reported and the exact commands you ran. Then stop.

> Never infer a "guard gap" from a red baseline. If validators are already
> failing on the trunk, a new failing fixture proves nothing about whether
> a verifier is too loose — it could just be the trunk being broken.

### Step 3 — create your candidate

Make the smallest possible change. One file is best; five is the hard cap.

### Step 4 — verify your fixture(s)

```sh
./bin/probe fixtures --json
```

For each fixture under `fixtures/positive/**` or `fixtures/negative/**`:

- A `<name>.meta.json` sidecar is required. Schema:
  ```json
  {
    "fixture_type": "decision" | "ir-aggregate" | "ir-event" | "query",
    "expected": "pass" | "fail",
    "expected_error_category": "schema_violation" | "validation_error" | "..." (optional),
    "expected_error_contains": "substring the error must contain" (optional, when expected=fail),
    "from_role": "dumb-agent",
    "purpose": "what THIS fixture proves — must be unique and meaningful (D015, D017, D018, D020, D023)"
  }
  ```
- Supported `fixture_type` values (each extension was its own decision —
  `decision` from D005, `ir-aggregate`/`ir-event`/`query` from D007):
  - `decision` — validates against `decisions/schema.riido.json`.
  - `ir-aggregate` / `ir-event` / `query` — validates against
    `ir/schema/ir.schema.json` AND the kind must match the meta type
    (a `kind: query` doc under `fixture_type: ir-aggregate` fails).
- Adding a new `fixture_type` requires its own decision; do not edit
  the verifier yourself.
- positive fixture under `fixtures/positive/` must declare
  `expected: "pass"` and must validate.
- negative fixture under `fixtures/negative/` must declare
  `expected: "fail"`; the actual validation error must contain
  `expected_error_contains` when set.

#### Purpose rules (D015 + D017 + D018 + D020 + D023 + D026)

`meta.purpose` is REQUIRED and is the field that decides whether your
fixture adds coverage or is duplicate noise. Four layers of enforcement,
applied in order; the first one that rejects wins:

1. **Non-empty (D015)**: `purpose` must be a non-empty, non-whitespace
   string. Missing purpose → fixture rejected.
2. **Structural noise marker (D023 + D026)**: rejected on FIRST
   occurrence (no banlist seeding required) when a `cycle-N` token
   (regex `(?i)\bcycle[\s-]\d+\b`) appears in ANY of:
   - `meta.purpose` raw text (D023)
   - the fixture filename (D026)
   - the raw data file content — catches cycle-N in title, examples,
     evidence.ref, anywhere (D026)
   - the raw meta file content outside purpose (D026)

   Also, for fixtures under `fixtures/*/decision/`, the data file's
   `id` must match `^000-fixture-` (D026 — supersedes D023's narrower
   999- ban). The 000-fixture- namespace is reserved for canonical
   designer-authored fixtures; any other prefix is the dumb-agent
   trying to look like a real decision (observed: 102-, 103-).
3. **Unique after normalization (D015 + D017)**: probe fixtures compares
   `NormalizePurpose(purpose)`, not the raw string. Normalization strips
   `(<digits>)` (Unix timestamps in parens), `cycle N` tokens, bare 6+-digit
   numbers, then lowercases. Two fixtures whose normalized purposes are
   equal are duplicates within `(category, fixture_type)`, even if their
   raw strings differ only by timestamp/cycle suffix. **Appending a
   timestamp to fake uniqueness does NOT work.**
4. **Not in banlist (D018 + D020 + future)**: `purpose-banlist.riido.json`
   holds known noise templates. Any fixture whose normalized purpose
   matches a banlist entry is rejected with a citation of the seeding
   decision — even if it's the only such fixture in the set. **Deleting
   the original does NOT reset the lockout.**

##### Fixture id convention (D023 + D026)

For files under `fixtures/*/decision/`:

- `000-fixture-*` — **the only allowed id prefix** (D026 made this
  exact, replacing D023's narrower 999- ban). Examples:
  `000-fixture-positive-minimal`, `000-fixture-negative-missing-owner`.
- Any other prefix (102-, 103-, 999-, 022-, …) — **rejected** as a
  noise marker. The dumb-agent was observed using 102- and 103- to
  look like real decisions; the whitelist eliminates that bypass.

Real decision records (under `decisions/`) keep using the sequential
`NNN-slug` numbering — only files under `fixtures/*/decision/` are
restricted.

To make `purpose` a meaningful coverage claim, write what the fixture's
content distinguishes from existing fixtures. Examples of GOOD purposes:

- `"decision missing scope field — verifier should report scope required"`
- `"work-item aggregate with empty slots array — schema requires at least one"`
- `"query result shape with returns-one instead of returns-list"`

Examples of BAD purposes (will be rejected):

- `"Ensure a unique accepted decision fixture validates successfully"`
  (banned, seeded by D018; template restatement of "verifier passes")
- `"Validate a unique accepted decision record in platform governance scope"`
  (banned, seeded by D020; wording variation of the D018 template)
- `"Verify accepted decision fixture remains valid for top-down
  governance scope in cycle 40"` (rejected by D023 noise marker —
  raw text contains `cycle 40`)
- `"Cycle 14 positive fixture"` (rejected by D023 noise marker on the
  raw `Cycle 14` token; would also normalize to "positive fixture" and
  trip layer 3 if the noise marker were absent)
- A fixture with purpose scrubbed of cycle-N but `examples` containing
  "for cycle 44" or filename containing `cycle-EPOCH` — D026 rejects
  on data-content / filename scan even when the purpose is clean
- `"Foo bar baz (1780485823)"` (timestamp stripped → "foo bar baz",
  matches every other fixture using the same template)

When agentguard rejects a purpose, the rejection message cites either
"duplicate purpose" or "banned purpose template — added by decision NNN".
Both mean the same thing: declare a coverage claim that no other fixture
already covers, not a wording variation of one.

### Step 5 — dry-run your diff

```sh
git diff --name-only main > /tmp/changed.txt
./bin/agentguard verify --role dumb-agent --stdin < /tmp/changed.txt
```

If violations exist, the output groups them as:

- **PR-level** (e.g. `PR touches N files, role allows at most 5`) — comes
  with a split-PR hint.
- **FORBIDDEN** — paths matching a forbidden_paths pattern. Re-classify
  or drop these files; never argue with a forbidden hit. The accompanying
  pattern is printed so you can find it in `agent-roles.riido.json`.
- **not allowed** — paths matching no allowed_paths pattern. The closest
  pattern is suggested. If the suggestion includes `would match if
  prefixed with "X"`, you are passing paths from the wrong base — fix
  the path or the pipeline (do not edit `agent-roles.riido.json`).

If multiple `not allowed` paths share the same missing prefix, agentguard
emits a single `prefix_drift` hint that says role config and diff producer
disagree on the path base. **This is a guard-author problem, not a
dumb-agent problem** — file a `reports/environment-blockers/<id>.md` note
describing what you observed and stop.

### Step 6 — re-probe the baseline

```sh
./bin/probe preflight
```

If your candidate change broke the baseline (e.g. you accidentally
modified an SSOT file you weren't supposed to), `baseline_blocked` will
fire here. Roll back the offending changes; never push a baseline-red PR.

### Step 7 — machine-readable mode for downstream tools

```sh
./bin/agentguard verify --role dumb-agent --stdin --json < /tmp/changed.txt
./bin/probe preflight --json
./bin/probe fixtures --json
```

Stable JSON schemas. Future tooling may parse them.

Agentguard schema:

```json
{
  "role": "dumb-agent",
  "ok": false,
  "files": ["..."],
  "violations": [
    {
      "path": "tools/x.go",
      "kind": "forbidden",
      "reason": "FORBIDDEN: matches forbidden_paths pattern \"tools/**\"",
      "matched_pattern": "tools/**"
    },
    {
      "path": "other/y.json",
      "kind": "not_allowed",
      "reason": "not allowed for role dumb-agent; closest allowed pattern is \"fixtures/positive/**\"",
      "closest_allowed": "fixtures/positive/**"
    },
    {
      "path": "",
      "kind": "max_files",
      "reason": "PR touches 40 files, role \"dumb-agent\" allows at most 5. Hint: ..."
    }
  ],
  "hints": ["prefix_drift: ..."]
}
```

Future agents should branch on `kind`, never parse `reason` strings.

## If you are the dumb-agent role

You are a **low-context closed-loop contract probe**. You are not a designer.
You are not a domain decision maker. You are not allowed to infer new business
meaning. You are not allowed to change accepted DSL, accepted IR schema,
accepted decisions, GitHub Actions, or verifier logic. Your job is to generate
small contract candidates and let tools decide.

Before every attempt, run the **Preflight** above.

### Allowed task types

| ID | Name                       | Where to write                                            |
|----|----------------------------|-----------------------------------------------------------|
| A  | positive fixture candidate | `fixtures/positive/<area>/<id>.json`                       |
| B  | negative fixture candidate | `fixtures/negative/<area>/<id>.json`                       |
| C  | fuzz corpus expansion      | `fuzz/corpus/<area>/<id>.json`                             |
| D  | manifest normalization     | `ir/candidates/<area>/<id>.json` (or in-place under fixtures/) |
| E  | failure minimization       | `reports/guard-candidates/<id>-min.md` with the minimized fixture inlined as a fenced JSON block |

Task E used to land under `reports/min/`, but Decision 004 narrowed
dumb-agent's writable space to `reports/guard-candidates/**` only. Embed
minimized fixtures in the guard-candidate markdown so a designer can promote
them to a real negative fixture (or to a verifier tightening) in a separate
guard-author PR.

### Allowed paths (enforced — repo-relative, no `contracts/` prefix)

```
fixtures/positive/**
fixtures/negative/**
fuzz/corpus/**
ir/candidates/**
reports/guard-candidates/**
reports/environment-blockers/**
```

### Forbidden paths (enforced — repo-relative)

```
tools/**
.github/workflows/**
decisions/**
dsl/core/**
ir/schema/**
ssot-dependency-map.riido.json
concept-map/concept-map.riido.json
agent-roles.riido.json
```

Anything not in allowed_paths and not in forbidden_paths is also rejected
(treated as not-allowed). When in doubt, write a `reports/guard-candidates/`
note instead of a file in some other location.

### Hard rules

- **At most 5 changed files per PR.** Enforced by agentguard. If you have more,
  split.
- **No mixing** of candidate-space changes with verifier changes in the same
  PR. Candidate PRs touch only candidate space. If you want to change a
  verifier, that is a different role (`guard-author`) and a different PR.
- **Never** modify a generated artifact by hand.
- **Never** delete tests, weaken validators, edit workflows, or modify
  accepted decisions.
- **Never** invent domain vocabulary. If a new word seems needed, create a
  guard-candidate note under `reports/guard-candidates/` instead.

### Bounded merge authority (decision 010)

Dumb-agent **may merge its own PRs** when ALL of the following hold:

1. `agentguard merge-check --role dumb-agent --stdin < changed.txt` returns
   `merge_allowed` (every file in `auto_merge_paths`, none in
   `forbidden_paths`, ≤ `max_files_per_pr`).
2. Every required CI check on the PR has passed (`verify`, `enforce-role`,
   `scan`, plus any role-specific checks).
3. Before merging: `probe preflight` on `main` returns `candidate_allowed`.
4. After merging: dumb-agent re-runs `probe preflight` on `main`. If status
   flips to `baseline_blocked`, dumb-agent must immediately stop further
   merges and file EITHER **one** `reports/environment-blockers/` note OR
   **one** revert PR — never both, never neither.

**Forbidden merge scope** (broader than the role's write forbidden_paths —
merge-check is stricter): `tools/**`, `.github/workflows/**`, `decisions/**`,
`dsl/**`, `ir/schema/**`, `agent-roles.riido.json`, `agent-roles.schema.json`,
`ssot-dependency-map.riido.json`, `concept-map/**`, `backend/**`,
`frontend/**`. Even if a future bug widens `allowed_paths`, the merge-check
predicate still blocks these because they're never in `auto_merge_paths`.

**ir/candidates/ is intentionally writable but NOT auto-mergeable.** Dumb-agent
may file candidate IR documents, but a designer reviews them before merge.

### Merge preflight one-liner

```sh
./bin/agentguard merge-check --role dumb-agent --stdin --json < /tmp/changed.txt
# {"role":"dumb-agent","allowed":true,"status":"merge_allowed","reasons":[],"files":[...]}
```

### Auto-merge (decision 012)

You don't have to call `gh pr merge` yourself. Open the PR with **either**:

- a label named `dumb-agent`, or
- a branch name starting with `dumb-agent/`

The `.github/workflows/dumb-agent-automerge.yml` workflow then runs:

1. Builds `agentguard`.
2. Computes the PR diff and runs `merge-check --role dumb-agent --json`.
3. If `merge_allowed`, calls `gh pr merge --auto --merge`. GitHub merges
   the PR as soon as every required check (verify, scan, enforce-role,
   contracts-drift in consumers) passes.
4. If `merge_blocked`, leaves a single PR comment with the reasons. You
   either fix the PR or accept that a designer must review.

This DOES NOT skip any check — required checks remain merge gates. It
only decides whether to enable auto-merge.

### How to think about failure

- Your candidate failed verifier? → Don't "fix" the verifier. Minimize the
  failing case and submit it as a `reports/guard-candidates/<id>-min.md` so
  a designer can decide whether the verifier or the candidate is wrong.
  **But first: confirm `probe preflight` was green BEFORE your change.**
  If the baseline was already red, a failing candidate proves nothing.
- The verifier accepted something that "looks weird"? → Don't change the
  verifier. Write a `reports/guard-candidates/<id>.md` describing the
  unexpected acceptance. A designer will decide whether to tighten. Again,
  this only counts if `probe preflight` was green when you started.
- `probe preflight` returned `baseline_blocked`? → Do NOT file a
  guard-candidate report. File at most one
  `reports/environment-blockers/<id>.md` describing the blocker. The
  failing baseline is an environment problem, not a domain gap.
- agentguard rejected a path you expected to be allowed? → Run
  `agentguard show --role dumb-agent` and compare. If the role config
  really doesn't allow it, your candidate is in the wrong place — move it
  or drop it. If the role config seems wrong, write a
  `reports/environment-blockers/<id>.md` describing the discrepancy and
  stop. **Never edit `agent-roles.riido.json` yourself.**

### Required PR template (mental)

```
Observed:
Attempt:                 [A | B | C | D | E]
Changed:                 [file list]
Verifier output:         [exact output]
Result:                  [pass | fail]
If failed, minimal case: [or "n/a"]
If passed, why mergeable:
Next:
```

If you cannot fill these in, the PR is not ready.

## If you are the guard-author role

You modify verifier code, schemas, tests, or workflows. You may NOT touch
fixtures, fuzz corpus, or IR candidates in the same PR — that would let you
quietly tune a verifier to make a specific candidate pass.

See `agent-roles.riido.json` for the exact allowed/forbidden paths.

## If you are the designer role

You can touch anything. Manual review and all other CI gates still apply.
agentguard does not block you, but other workflows (contracts.yml, security.yml,
dumb-agent-safety.yml) still run.
