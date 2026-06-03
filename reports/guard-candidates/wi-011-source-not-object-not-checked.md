# Guard candidate: WorkItem source object can be non-object

Added `fixtures/negative/work-item/work-item-invalid-source-type-not-object.json` where `source` is a string instead of the expected object.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating this malformed shape is not rejected in the local verifier surface.
