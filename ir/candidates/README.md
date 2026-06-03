# IR candidates

Candidate IR documents that have **not** been promoted to `ir/domain/`.

A candidate is dropped here when:

- A dumb agent generated a JSON IR variation that needs verifier review.
- A designer is experimenting with an IR shape before deciding whether to
  promote it to an accepted aggregate.

Anything in this directory:

- Does **not** participate in `irdrift` checks (only `ir/domain/` files do).
- Is **not** part of SSOT.
- May be deleted at any time by a designer.

Promotion path: a designer reviews a candidate, adds the corresponding DSL in
`dsl/domain/`, runs the emitter, and the new IR appears under `ir/domain/`.
The candidate file is then removed.
