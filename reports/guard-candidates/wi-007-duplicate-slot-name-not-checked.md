# Guard candidate: duplicate slot names in WorkItem are not rejected

Added `fixtures/negative/work-item/work-item-invalid-duplicate-slot-name.json` where the `slots` array contains duplicate slot `name` values (`id`).

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating duplicate slot names are not validated for `work-item` in the local verifier surface.
