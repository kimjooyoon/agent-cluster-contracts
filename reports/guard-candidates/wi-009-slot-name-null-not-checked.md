# Guard candidate: slot name explicitly null is not rejected

Added `fixtures/negative/work-item/work-item-invalid-slot-name-null.json` with one `slots` entry having `"name": null`.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating null slot names are not validated for WorkItem in the local verifier surface.
