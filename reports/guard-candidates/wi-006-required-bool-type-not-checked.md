# Guard candidate: fixture with non-boolean required flag is not rejected

Added `fixtures/negative/work-item/work-item-invalid-required-bool-type.json` where `slots[0].required` is the string `"yes"` (expected boolean).

Local verifier run:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands pass, indicating local verifier checks still do not validate negative fixture inputs under `fixtures/negative/**`.
