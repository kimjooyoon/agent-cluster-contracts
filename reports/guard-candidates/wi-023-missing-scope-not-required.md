# Guard candidate: decision fixture missing scope is currently accepted

Observed on `probe fixtures`:
- `fixtures/positive/decision/decision-pass-missing-scope.json` with
  `expected: pass` passed validation.
- The `scope` field is optional in the decision schema, so this is valid behavior.

Action taken in this PR:
- Added `fixtures/positive/decision/decision-pass-missing-scope.json` + sidecar as a passing fixture.
- Added this note so this behavior is recorded for future policy decisions.
