# Guard candidate: fuzz WorkItem entry not exercised by local verifier stack

Added a fuzz corpus example at `fuzz/corpus/work-item/work-item-required-bool-bug.json` with an intentionally malformed slot shape (`required` set to a string for `invalid_required_field`) and a non-canonical `fuzzer_note` field.

Observed local verifier result:
- `go build -o bin/ ./tools/...`
- `./bin/ssotdeps verify`
- `./bin/conceptmap verify`
- `./bin/decision validate`
- `./bin/decision list --status accepted`
- `./bin/secretscan .`

All commands passed.

The current local verification surface does not validate `fuzz/corpus/**` inputs, so malformed fuzz fixtures are neither rejected nor surfaced by this check set.
