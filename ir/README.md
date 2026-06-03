# Intermediate Representation — JSON IR

Every aggregate, event, lifecycle state, policy, and view defined in the Common
Lisp DSL is compiled to JSON IR and stored under this directory.

## Layout (planned)

```
ir/
├── schema/
│   └── ir.schema.json      JSON Schema describing every IR document
├── domain/
│   └── <context>.ir.json   Generated, one file per bounded context
└── fixtures/
    └── <context>/*.json    Generated contract fixtures consumed by backend tests
```

The schema and the actual IR files appear only with the first vertical slice
(Phase 3). This directory currently holds only this README.

## Rules

- **IR is generated. Never edit `domain/*.ir.json` or `fixtures/*.json` by hand.**
- Every IR file has a `source.dsl_file` and `source.sha256` field. `tools/ssotdeps
  verify` re-emits the DSL and rejects merges where the IR drifts from its source.
- Consumers (backend, frontend) read IR through generated code, never by parsing
  it directly. The generator lives in `tools/` (to be added with the first slice).
- IR vocabulary is the only vocabulary backend and frontend may use for domain
  concepts. If a consumer needs a new word, escalate to DSL via a decision record.
