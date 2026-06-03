# Guard candidate: fixture-path validation is not exercised by local verifier commands

Added `fixtures/negative/work-item/work-item-invalid-missing-slots.json` containing an intentionally invalid WorkItem fixture (`source.sha256` omitted).

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All pass. This indicates the current local verifier surface still does not validate `fixtures/negative/**` inputs.
