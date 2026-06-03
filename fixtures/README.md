# Fixtures

Candidate space for the `dumb-agent` role.

- `positive/` — inputs that **must pass** the relevant verifier (decision
  records that validate, concept-map fragments that validate, etc.).
- `negative/` — inputs that **must fail** with an expected error.

Each fixture is a JSON file plus, optionally, an adjacent `.meta.json`
declaring the verifier to run and the expected outcome:

```json
{
  "verifier": "decision validate",
  "expected": "pass",
  "from_role": "dumb-agent",
  "supersedes": null
}
```

Fixtures here are **not** SSOT. They are the inputs that exercise the
verifiers. Modifying a verifier to make a fixture pass that previously failed
is forbidden in the same PR (see `AGENT_CONTRACT.md`).
