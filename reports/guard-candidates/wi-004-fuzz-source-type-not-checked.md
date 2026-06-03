# Guard candidate: fuzz corpus malformed source field is not covered

Added `fuzz/corpus/work-item/work-item-invalid-source-type.json` with an intentionally malformed WorkItem corpus entry (`source` is a string, not an object).

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All pass. The local verifier set does not consume `fuzz/corpus/**` inputs.
