# Reports

Notes and minimal failing cases produced by agents during candidate runs.

- `min/<id>.json` — minimized failing fixture (output of task type E).
- `guard-candidates/<id>.md` — markdown note describing a verifier weakness or
  an unexpected acceptance (output when the dumb agent finds something
  surprising). Designers triage these into either a verifier tightening
  (separate guard-author PR) or a new positive/negative fixture.
- `runs/<date>/<id>.json` — telemetry from one dumb-agent invocation (optional).

Files here are not SSOT; they are evidence. Treat them as input for future
decisions, not as decisions themselves.
