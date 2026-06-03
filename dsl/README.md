# DSL — Common Lisp

The project's domain language is Common Lisp source files in this directory.
They are read by a Lisp-based reader (SBCL) and emitted as JSON IR into `../ir/`.

## Why Common Lisp

- Homoiconic. The DSL surface is plain Lisp s-expressions, no parser layer.
- Macros let domain-specific forms (`defaggregate`, `defevent`, `deflifecycle`,
  `defpolicy`, `defview`) compile to JSON IR without a separate transformation
  language.
- SBCL is available on every developer machine in this project (`/opt/homebrew/bin/sbcl`).

## Files (planned)

| File                       | Purpose                                          |
|----------------------------|--------------------------------------------------|
| `core.lisp`                | DSL macros: `defaggregate`, `defevent`, etc.    |
| `emit-ir.lisp`             | Walks defined forms and writes JSON IR.         |
| `domain/*.lisp`            | Per-bounded-context DSL definitions.            |

The DSL files are not present yet — they belong to the first vertical slice
(Phase 3), recorded as a separate decision before implementation.

## Workflow

```
dsl/domain/<context>.lisp   →   sbcl --script dsl/emit-ir.lisp   →   ir/<context>.ir.json
```

Generated IR is never edited by hand. If the IR is wrong, fix the DSL or the
emitter, then regenerate.

## Rules

- DSL adds vocabulary only when a decision record says so.
- DSL must always emit deterministic IR (same input → byte-identical output).
- `tools/ssotdeps verify` checks that every IR file has a current DSL source.
