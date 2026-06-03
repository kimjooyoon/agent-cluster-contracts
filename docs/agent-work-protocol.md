# Agent Work Protocol

This protocol is the **operational interpretation** of the initial agreement
(`initial-agreement.md` / decision 001) for any agent (Claude, Codex, or future)
working on the project.

It is binding by reference: if a PR violates this protocol, the PR is rejected.
The mechanical guards in `tools/` and `.github/workflows/` cover the parts of this
protocol that can be checked automatically. The unchecked parts rely on agent
discipline plus human review.

---

## 0. Before any change

1. Read the relevant SSOT files. Do not infer them.
2. Query the knowledge index: `tools/knowledge query --q "<question>"`.
3. Identify the **owner repo**. If unclear, do not start; create a decision record
   or a conflict record first.
4. Identify the **kind of work**:
   - new decision
   - implementation of existing decision
   - conflict
   - repeated reasoning (you have reasoned this way before)
   - generated artifact drift
   - guard candidate

If the kind is "repeated reasoning", stop and record a decision before continuing.

## 1. Selecting the source pattern

For each new work item:

```sh
tools/devpattern select --work-item <WI-id>
```

This writes a JSONL event recording `top_down` or `bottom_up`. You follow the
selected pattern for that work item. Do not override the selection silently. If
random selection produces an unworkable assignment, record a decision and evolve
`devpattern` — do not just pick the other one.

## 2. SSOT-first edit order

```
DSL                            (only when concept itself is new or changed)
  → IR                         (generated)
    → generated fixtures / code (generated)
      → consumer implementation (backend/frontend)
        → tests
          → CI / pre-commit guard
```

You **never** edit generated artifacts as if they were SSOT. If something is wrong
in `ir/`, fix the DSL or the emitter, then regenerate.

If a consumer needs a new word or concept, stop and escalate to DSL via a decision
record. Do not add the word locally to the consumer repo.

## 3. Decision records

Every non-trivial change carries a decision record. Use:

```sh
tools/decision create --id <NNN-slug> --title "..." --status proposed \
  --source top_down|bottom_up --owner <gh-login>
tools/decision explain <NNN-slug>
tools/decision list --status accepted
tools/decision supersede --old <NNN-old> --new <NNN-new>
```

A decision is **weak** if it does not remove future ambiguity (mark `weak: true`
in the record). Weak decisions never block merges.

A conflict between two existing decisions does not get silently resolved. Create
`<NNN>-conflict-<a>-vs-<b>` as a new decision and either pick one (with evidence)
or supersede both with a third.

## 4. Constraints

A constraint becomes active only when at least one of these is true:

- it is enforced by DSL (form rejected at compile time);
- it is enforced by a Go tool in `tools/` that exits non-zero on violation;
- it is enforced by a GitHub Action that fails the merge;
- it is enforced by a pre-commit hook;
- it is recorded in `concept-map.riido.json` as a prose rule **with at least one
  example and at least one counterexample**, and has a `guard_candidate` field.

Adding a rule "because it feels right" without one of the above is not allowed.

## 5. Generated artifacts

- Generated files declare their source (path + sha256).
- `tools/ssotdeps verify` re-derives the source for every generated file and fails
  on drift.
- Generated artifacts are kept in the consumer repo when consumer-specific and in
  contracts when contract-wide (fixtures).

## 6. Security

- Never commit a raw secret. `tools/secretscan` and `.github/workflows/security.yml`
  enforce this on every commit and every PR.
- When a workflow needs a secret, document only the stable key name (e.g.
  `SLACK_BOT_TOKEN`). Never document the value.
- Do not paste tokens into logs, decision records, research ledgers, the knowledge
  index, fixtures, or generated docs.

## 7. PR template (mental)

Every PR description answers:

- **Decision id:** `NNN-slug` (or "weak: <reason>", or "no decision needed: <reason>")
- **Source pattern:** `top_down` | `bottom_up` (from devpattern event)
- **Affected SSOT:**
- **Generated artifacts:**
- **Verification commands:** the exact commands a reviewer runs to confirm
- **What future decision this removes:**

If you cannot fill these in, the change is not ready for review.

## 8. When the contract is wrong

If implementation reveals the contract itself is wrong:

1. Record bottom-up evidence in the new decision (`source: bottom_up`).
2. Add at least one example and one counterexample.
3. Change the contract first (concept map, DSL, schema, dep map).
4. Regenerate downstream artifacts.
5. Mark the older decision as `superseded` with `superseded_by: <new-id>`.

Do not implement a workaround in the consumer repo and leave the contract stale.

## 9. Output style for agent reports

When summarizing work, use:

- **Observed:**
- **Decision:**
- **SSOT owner:**
- **Changed:**
- **Generated:**
- **Verified:**
- **Remaining:**
- **Next:**

Vague phrases like "implemented feature" are not acceptable. Prefer "moved this
decision into this SSOT and protected it with this check."
