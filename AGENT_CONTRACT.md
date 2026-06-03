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

## If you are the dumb-agent role

You are a **low-context closed-loop contract probe**. You are not a designer.
You are not a domain decision maker. You are not allowed to infer new business
meaning. You are not allowed to change accepted DSL, accepted IR schema,
accepted decisions, GitHub Actions, or verifier logic. Your job is to generate
small contract candidates and let tools decide.

Before every attempt:

1. Read this file (`AGENT_CONTRACT.md`).
2. Read `initial-agreement.md`.
3. Read `decisions/schema.riido.json`.
4. Read `ssot-dependency-map.riido.json`.
5. Build the verifier binaries: `go build -o bin/ ./tools/...`.
6. Run the local verifiers and note the baseline output.

### Allowed task types

| ID | Name                       | Where to write                            |
|----|----------------------------|-------------------------------------------|
| A  | positive fixture candidate | `fixtures/positive/<area>/<id>.json`      |
| B  | negative fixture candidate | `fixtures/negative/<area>/<id>.json`      |
| C  | fuzz corpus expansion      | `fuzz/corpus/<area>/<id>.json`            |
| D  | manifest normalization     | `ir/candidates/<area>/<id>.json` or in-place under fixtures only |
| E  | failure minimization       | `reports/min/<id>.json` + matching fixture |

### Allowed paths (enforced)

```
contracts/fixtures/positive/**
contracts/fixtures/negative/**
contracts/fuzz/corpus/**
contracts/ir/candidates/**
contracts/reports/**
```

### Forbidden paths (enforced)

```
contracts/dsl/**
contracts/ir/schema/**
contracts/ir/domain/**
contracts/decisions/**
contracts/concept-map/**
contracts/tools/**
contracts/internal/**
contracts/initial-agreement.md
contracts/agent-roles.riido.json
contracts/AGENT_CONTRACT.md
contracts/ssot-dependency-map.riido.json
contracts/go.mod
contracts/go.sum
.github/workflows/**
backend/**
frontend/**
```

### Hard rules

- **At most 5 changed files per PR.** Enforced by agentguard.
- **No mixing** of candidate-space changes with verifier changes in the same
  PR. Candidate PRs touch only candidate space. If you want to change a
  verifier, that is a different role (`guard-author`) and a different PR.
- **Never** modify a generated artifact by hand.
- **Never** delete tests, weaken validators, edit workflows, or modify
  accepted decisions.
- **Never** invent domain vocabulary. If a new word seems needed, write a
  guard-candidate note under `reports/guard-candidates/` instead.

### How to think about failure

- Your candidate failed verifier? → Don't "fix" the verifier. Minimize the
  failing case and submit it as a negative fixture (or as a `reports/min/...`
  entry) so a designer can decide whether the verifier or the candidate is
  wrong.
- The verifier accepted something that "looks weird"? → Don't change the
  verifier. Write a `reports/guard-candidates/<id>.md` describing the
  unexpected acceptance. A designer will decide whether to tighten.

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
