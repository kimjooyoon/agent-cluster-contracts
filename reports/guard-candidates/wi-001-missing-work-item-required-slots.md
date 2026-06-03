# Guard candidate: unvalidated negative WorkItem fixture

A candidate was added at `fixtures/negative/work-item/work-item-invalid-missing-required.json` with a WorkItem IR shape that omits required `slots` (`title`, `state`) from the canonical `ir/domain/work-item.ir.json` shape.

Run result: local verifier commands passed unchanged:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

This indicates the current contract verification surface does not exercise negative fixture files under `fixtures/negative/**`, so malformed WorkItem fixtures can be accepted at review time.
