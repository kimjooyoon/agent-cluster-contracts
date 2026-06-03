# Guard candidate: WorkItem slot missing `type` is not rejected

Added `fixtures/negative/work-item/work-item-invalid-slot-missing-type.json` with an intentionally invalid WorkItem fixture where a slot object omits the required `type` field.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating malformed WorkItem fixtures under `fixtures/negative/work-item/**` with missing slot `type` are not currently validated.
