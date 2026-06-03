# Guard candidate: slot missing `name` is not rejected

Added `fixtures/negative/work-item/work-item-invalid-slot-missing-name.json` where one `slots` entry omits `name`.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating a slot object missing `name` is not rejected for WorkItem in the local verifier surface.
