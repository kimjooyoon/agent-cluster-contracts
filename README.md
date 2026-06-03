# agent-cluster-contracts

**Single source of truth for the agent_cluster_project platform.**

This repo owns canonical vocabulary, the Common Lisp DSL, the JSON IR, decision
records, the concept map, the SSOT dependency map, constraint definitions, generated
contract fixtures, and the verifier tools that protect contract drift.

No other repo redefines anything that lives here.

## Layout

```
.
├── initial-agreement.md          Project's first contract (frozen Section 0)
├── dsl/                          Common Lisp DSL sources
├── ir/                           JSON IR schemas and generated IR (see ir/README.md)
├── decisions/                    Decision records: YYYY/MM/DD/<id>.decision.riido.json
│   └── schema.riido.json
├── concept-map/
│   ├── schema.riido.json
│   └── concept-map.riido.json    The single concept map (split only with evidence)
├── ssot-dependency-map.riido.json
├── docs/
│   ├── agent-work-protocol.md    The per-work-item protocol agents follow
│   └── research/
│       ├── source-ledger.riido.json
│       └── pending-research.md
├── tools/                        Six Go binaries (see tools/README.md)
│   ├── decision/
│   ├── knowledge/
│   ├── ssotdeps/
│   ├── conceptmap/
│   ├── devpattern/
│   └── secretscan/
├── internal/                     Shared Go packages
├── .githooks/                    Pre-commit hook scripts
└── .github/workflows/            contracts.yml + security.yml
```

## How to use this repo

```sh
go build -o bin/ ./tools/...
./bin/ssotdeps verify
./bin/conceptmap verify
./bin/decision list --status accepted
./bin/secretscan .
```

If any of those exit non-zero, do not merge.

## How to extend

1. Read `docs/agent-work-protocol.md`.
2. Query `./bin/knowledge query --q "..."` for prior decisions.
3. Update SSOT (DSL, IR, decision record) **before** any consumer code.
4. If a new constraint is needed, add it to `concept-map/concept-map.riido.json`
   with at least one example, then create the matching decision record and (if the
   rule is detectable) the matching Go verifier.

See `initial-agreement.md` for the full project contract.
