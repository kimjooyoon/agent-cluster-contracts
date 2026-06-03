# Guard candidate: WorkItem fixture with non-string `name` is not rejected

Added `fixtures/negative/work-item/work-item-invalid-name-type.json` where `name` is a number (`123`) instead of a string.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating numeric `name` is not rejected for WorkItem in the local verifier surface.
