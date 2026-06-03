# Guard candidate: malformed WorkItem slots field not validated by local verifiers

Added `fixtures/negative/work-item/work-item-invalid-slots-type.json` with an intentionally malformed WorkItem fixture where `slots` is an object instead of an array.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating current local checks do not validate this `fixtures/negative/**` input.
