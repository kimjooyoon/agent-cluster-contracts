# tools/ — verifier and probe binaries

Every command listed here is a Go binary in this repo. Build once:

```sh
go build -o bin/ ./tools/...
```

| Tool | Subcommands | Dumb-agent may run? | Smart-agent only? |
|------|-------------|---------------------|-------------------|
| [decision](#decision)        | list, explain, create, validate, supersede | read-only commands yes (list, explain, validate); **create/supersede are smart-agent only** | yes for write |
| [knowledge](#knowledge)      | ingest, query, record, supersede, list     | yes (read) | yes for `record`/`supersede` writing to .knowledge/ |
| [ssotdeps](#ssotdeps)        | verify, show                                | yes (`verify --mode local`); never modify the SSOT itself | yes for `--mode full` interpretation when sibling repos must be checked |
| [conceptmap](#conceptmap)    | verify, query, list                         | yes (read-only) | yes for any edit to the concept map |
| [devpattern](#devpattern)    | select, list, summary                       | yes (smart-agent typically owns this) | recommended; dumb-agent may run select but should not alter records |
| [secretscan](#secretscan)    | (default scan)                              | yes (read-only) | n/a (scan is read-only) |
| [irdrift](#irdrift)          | (default check)                             | yes (read-only) | yes for fixing drift (regenerating IR is a smart-agent task) |
| [agentguard](#agentguard)    | verify, list, show                          | **yes — start every PR with `agentguard show --role dumb-agent`** | smart-agent only for editing agent-roles.riido.json |
| [probe](#probe)              | preflight, fixtures                         | **yes — start every PR with `probe preflight`** | n/a |
| [gen-go-client](#gen-go-client)   | (default)                              | **no — forbidden_paths include tools/**             | yes |
| [gen-dart-client](#gen-dart-client) | (default)                            | **no**                                              | yes |

Tools marked **dumb-agent may run** can be executed from a dumb-agent's
preflight/postflight scripts. Tools that write to `agent-roles.riido.json`,
`ssot-dependency-map.riido.json`, `decisions/**`, `dsl/**`, `ir/domain/**`,
`ir/schema/**`, or `tools/**` themselves are **smart-agent only**, and the
agentguard role enforces this at the path level.

---

## decision

Manage decision records under `decisions/YYYY/MM/DD/<id>.decision.riido.json`.

```sh
./bin/decision list [--status accepted|proposed|superseded|rejected] [--json]
./bin/decision explain <id> [--json]
./bin/decision create   --id NNN-slug --title "..." --source top_down|bottom_up \
                        --owner gh-login --example "..." --evidence kind=ref \
                        [--status proposed] [--area a,b] [--repo R1,R2] \
                        [--ssot R] [--weak] [--date YYYY-MM-DD]
./bin/decision validate
./bin/decision supersede --old <id> --new <id>
```

Exit 1 on validation failure. **dumb-agent must not call `create` or
`supersede`** — those decisions belong to a smart agent.

## knowledge

File-based inverted index over `.md` / `.json` / `.lisp` / `.go` / `.yml`.

```sh
./bin/knowledge ingest    [--extra ../backend,../frontend]
./bin/knowledge query     --q "..." [--top N] [--agent claude|codex] [--json]
./bin/knowledge record    --id ID --title "..." [--kind ...] [--body ...]
./bin/knowledge supersede --old ID --new ID [--note ...]
./bin/knowledge list      [--json]
```

The index is rebuildable; treat it as a cache, never as SSOT.

## ssotdeps

```sh
./bin/ssotdeps verify [--mode local|full] [--json]
./bin/ssotdeps show
```

`--mode local` (default for dumb-agent via `probe preflight`) skips all
sibling-repo (backend, frontend) checks even when those repos are checked
out. Use this when you do not control whether sibling repos are present.

`--mode full` (default for `./bin/ssotdeps verify`) also checks
sibling-repo consumer paths and CI gates when those repos exist. Used by
smart-agent local dev and full cross-repo CI. **Do not weaken this.**

## conceptmap

```sh
./bin/conceptmap verify
./bin/conceptmap query <ConceptName>
./bin/conceptmap list [--json]
```

Read-only; the concept map itself is a forbidden_path for dumb-agent.

## devpattern

```sh
./bin/devpattern select --work-item ID [--override top_down|bottom_up] [--note "..."] [--json]
./bin/devpattern list   [--work-item ID] [--json]
./bin/devpattern summary
```

Crypto-random `top_down|bottom_up` selection per work item; events append to
`.devpattern/events.jsonl`. Smart-agent normally calls this; dumb-agent may
read but should not invent records.

## secretscan

```sh
./bin/secretscan [PATH...]
```

Default scans the current directory. Append `secretscan:ignore` on a line
to suppress matches. Exit 1 on any finding. Honored by every repo's
`security.yml`.

## irdrift

```sh
./bin/irdrift [--json]
```

Re-runs the SBCL emitter (`dsl/emit-ir.lisp`) in a temp dir and diffs
against the committed `ir/domain/*.ir.json`. Exit 1 on drift. Requires
`sbcl` and either `sha256sum` (Linux) or `shasum` (macOS) on PATH.

## agentguard

Per-role path-allowlist enforcement for PR diffs.

```sh
./bin/agentguard show   --role ROLE [--json]
./bin/agentguard verify --role ROLE [--files A,B | --stdin | --from REF [--to REF]] [--json]
./bin/agentguard list
```

**Every dumb-agent PR starts with `agentguard show --role dumb-agent` to
read the rules and ends with `agentguard verify --role dumb-agent --stdin`
to confirm the diff complies.** See `AGENT_CONTRACT.md`.

Diagnostics group by kind: `max_files`, `forbidden`, `not_allowed`. JSON
schema is stable; future tooling may parse it.

## probe

Dumb-agent baseline + fixture verifier (introduced by decision 005).

```sh
./bin/probe preflight [--json]
./bin/probe fixtures  [--json]
```

`preflight` runs every contracts-local baseline check and reports one of:

- `candidate_allowed` → dumb-agent may proceed (open exactly one small
  candidate PR; max 5 files per agent-roles).
- `baseline_blocked` → dumb-agent **must stop**. Optionally file one
  `reports/environment-blockers/<id>.md` describing what is broken; never
  create domain fixtures or guard-candidate notes under a red baseline.

`fixtures` walks `fixtures/{positive,negative}/**` and verifies each
fixture against its `.meta.json` sidecar. v1 supports `fixture_type:
"decision"` only; other types require their own decision.

## gen-go-client

```sh
./bin/gen-go-client [--ir-dir <dir>] --out-dir <dir> [--package contracts]
```

Reads `ir/domain/*.ir.json` and emits gofmt-clean Go structs into the
target dir. Output is the canonical generated client for
`agent-cluster-backend`. Backend's `contracts-drift.yml` re-runs this in
CI and diffs against committed files.

## gen-dart-client

```sh
./bin/gen-dart-client [--ir-dir <dir>] --out-dir <dir>
```

Same shape as gen-go-client but emits Dart classes for
`agent-cluster-frontend`. Frontend's `contracts-drift.yml` re-runs this
in CI and diffs against committed files.

---

## Where each tool's source lives

| Tool | Source |
|------|--------|
| decision        | `tools/decision/main.go`        + `internal/decision/` |
| knowledge       | `tools/knowledge/main.go`       + `internal/knowledge/` |
| ssotdeps        | `tools/ssotdeps/main.go`        + `internal/ssotdeps/` |
| conceptmap      | `tools/conceptmap/main.go`      + `internal/conceptmap/` |
| devpattern      | `tools/devpattern/main.go`      + `internal/devpattern/` |
| secretscan      | `tools/secretscan/main.go`      + `internal/secretscan/` |
| irdrift         | `tools/irdrift/main.go`         + `internal/irdrift/` |
| agentguard      | `tools/agentguard/main.go`      + `internal/agentguard/` |
| probe           | `tools/probe/main.go`           + `internal/probe/` |
| gen-go-client   | `tools/gen-go-client/main.go`   + `internal/codegen/` |
| gen-dart-client | `tools/gen-dart-client/main.go` + `internal/codegen/` |
