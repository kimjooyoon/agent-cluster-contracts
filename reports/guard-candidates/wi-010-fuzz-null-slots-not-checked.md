# Guard candidate: fuzz corpus with `slots: null` is not rejected

Added `fuzz/corpus/work-item/work-item-invalid-slots-null.json` where `slots` is explicitly `null`.

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating `slots: null` is not rejected for WorkItem in the local verifier surface.

Also observed:
- `./bin/agentguard verify --role "dumb-agent" --stdin < /tmp/changed.txt` reports the path pattern mismatch for the nested repository (existing repo-structural condition), but this did not affect the local verifier command stack.
