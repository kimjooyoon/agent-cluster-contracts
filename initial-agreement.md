# Initial Agreement — agent_cluster_project

**Status:** accepted (frozen contract for the project bootstrap)
**Decision record:** `decisions/2026/06/03/001-initial-agreement.decision.riido.json`
**Date:** 2026-06-03

This document is the human-readable mirror of decision **001-initial-agreement**.
If the two disagree, the JSON decision record is authoritative.

---

The project follows DDD and bounded context discipline.

The system is expected to change continuously, but constraints are not inferred
freely. A constraint becomes active only when it is expressed as DSL, IR, generated
check, local tool, GitHub Action, pre-commit hook, or documented evidence-backed
rule.

The system does not rely on agent judgment to protect boundaries. Boundaries are
protected by tools, generated artifacts, tests, and GitHub Actions that reject
merge when contracts drift.

The agent may reason while working, but repeated reasoning should be converted into
a recorded decision and then into a detectable rule or guard.

If similar reasoning happens twice, or if a similar problem appears twice, record
it. If a recorded issue can be detected without reasoning, add a local Go tool and
a GitHub Action guard. If it cannot yet be expressed as DSL, record examples and
counterexamples first. Do not add a new constraint from irritation or preference
alone.

Every major change should answer:

- Which SSOT owns this?
- Which repo consumes this?
- Which generated artifact depends on this?
- Which CI gate protects this?
- Which future repeated decision is removed by this change?

## Technology contract

| Surface                  | Choice                                            |
|--------------------------|---------------------------------------------------|
| DSL                      | Common Lisp                                       |
| Constraint language      | DSL first; evidence-backed prose only when DSL cannot express the rule yet |
| Intermediate representation | JSON IR                                        |
| Implementation language  | Go                                                |
| UI                       | Flutter, Material Design, Atomic Design           |
| Structured ops           | GraphQL                                           |
| Real-time events         | SSE                                               |
| Deployment               | Local first; Terraform structured for later cloud |
| Secrets                  | GitHub Secrets; never committed                   |
| Notifications            | Slack                                             |
| Workspaces               | GitHub repositories                               |

## Repository split

- `agent-cluster-contracts` — vocabulary, DSL, IR, decision records, concept map,
  SSOT dep map, fixtures, verifier tools.
- `agent-cluster-backend` — Go backend, GraphQL, SSE, local runtime, contract
  consumption.
- `agent-cluster-frontend` — Flutter UI, Material, Atomic Design, generated client
  consumption.

No repo redefines vocabulary owned by contracts.

## SSOT direction

```
Common Lisp DSL → JSON IR → generated schema / fixture → generated code or client
                                                       → implementation → tests / CI / pre-commit
```

Generated artifacts are not edited as SSOT. When generated artifacts drift, change
the DSL or generator, then regenerate.

## Concept map

Start with one concept-map surface owned by contracts. Split it only when evidence
shows that one map creates drift, ownership conflict, or query ambiguity. Each
concept-map rule includes at least one example. A constraint cannot be added only
because someone feels discomfort.

## Decision record

Every non-trivial decision is recorded under
`decisions/YYYY/MM/DD/<id>.decision.riido.json` and conforms to
`decisions/schema.riido.json`. A weak decision (one that does not remove future
ambiguity) is marked `weak: true` and does not block merges.

## Development pattern

For each new work item, `tools/devpattern select` chooses `top_down` or `bottom_up`
randomly and records the selection. The agent follows the selected pattern. If the
random choice later proves harmful, evolve the tool by decision record — do not
override silently.

## Security

Secrets live in GitHub Secrets or local developer secret stores. Raw tokens, API
keys, credential files, private endpoints, live deployment evidence, smoke payloads
containing secrets, and logs containing authorization headers are never committed.
`tools/secretscan` and `.github/workflows/security.yml` enforce this.
