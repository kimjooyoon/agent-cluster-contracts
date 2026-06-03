# Environment blockers

Notes filed by an agent (typically dumb-agent) when `probe preflight` returns
`baseline_blocked` and the dumb-agent cannot proceed. **Not** for domain
verifier gaps.

## When to file one

Only when **all** of these are true:

1. `probe preflight` returned `status: baseline_blocked`.
2. The blocker is environmental: missing tool on PATH (sbcl, go), missing
   `tools/README.md`, broken pre-existing fixture/IR/dep map, missing sibling
   repo for a check that should have been local-mode, etc.
3. You have not yet filed one for the same blocker today.

Decision 005 limits a dumb-agent to **at most one environment-blocker report
per probe attempt** under a red baseline. After filing it, stop.

## When NOT to file one

- Domain validation gaps (a candidate that probably should have been rejected
  by some verifier but wasn't) → `reports/guard-candidates/`.
- Anything you only suspect — only file things you observed in tool output.
- Anything that would require you to modify a forbidden path to "fix".

## Naming

`reports/environment-blockers/<id>-<short-kebab-slug>.md`

`id` is a fresh `eb-NNN` sequence local to this directory.

## Required body

Each note must include, in markdown:

- **Observed:** what `probe preflight` (or another tool) printed.
- **Suspected blocker:** the environmental issue you think this is (don't
  speculate beyond what the tool output supports).
- **Reproduction:** the exact commands you ran.
- **Files you did NOT touch:** confirm you wrote no fixtures, IR, or domain
  guard-candidate notes during this attempt.

A designer triages these into either a real environment fix (separate
guard-author PR) or a "no-op, environment was fine, your reasoning was off"
response.
