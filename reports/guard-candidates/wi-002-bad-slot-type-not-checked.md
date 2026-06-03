# Guard candidate: WorkItem negative fixture not covered by local verifier commands

Added `fixtures/negative/work-item/work-item-invalid-slot-type.json` with an intentionally invalid WorkItem IR fixture (`slots[0].type` is a number instead of a string).

Verifier commands run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, and no command in the current verifier set validates malformed files under `fixtures/negative/**`.
