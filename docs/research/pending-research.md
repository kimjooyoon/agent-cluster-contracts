# Pending Research

External web access was not used during the 2026-06-03 bootstrap. The following
topics from the initial agreement (Section 7) need verified citations before any
decision relies on a specific detail. Until then, claims about these tools are
written as weak knowledge in the knowledge index.

| Topic                                | Why we need it                                         | Affected decision |
|--------------------------------------|--------------------------------------------------------|-------------------|
| Claude Code latest hooks / slash commands | tool definitions in `tools/knowledge` may benefit | TBD               |
| Codex CLI / OpenAI coding agent invocation | so `tools/knowledge query --agent codex` matches actual behavior | TBD |
| GitHub Actions current syntax for matrix/reusable workflows | `contracts.yml` and `security.yml` use a conservative subset until verified | TBD |
| GitHub Secrets — org vs repo scoping best practice | Slack/notification secrets need a stable scoping decision | TBD |
| Go 1.26 toolchain features            | `go.mod` declares 1.26; verify recommended idioms     | TBD               |
| Flutter stable channel + GetX current status | frontend skeleton uses GetX placeholder; need real version pins | TBD |
| GraphQL Go server libs (gqlgen vs 99designs)| backend skeleton intentionally avoids choosing yet  | TBD               |
| SSE patterns in Go (chi/echo middleware) | backend skeleton intentionally avoids choosing yet   | TBD               |
| Common Lisp tooling (SBCL + Roswell / qlot) | DSL emitter design                                  | TBD               |
| Local RAG / embeddings options for `tools/knowledge` | currently a grep + index over files; vector layer is a future upgrade | TBD |

## How to convert a pending item to a verified one

1. Run web research and capture the source URL.
2. Add an entry to `source-ledger.riido.json` with `confidence` and a
   review/expiration date.
3. Open a decision record that cites the ledger entry.
4. Remove the row from this file (or move it to "verified" section).
5. If the research changes a contract, update DSL / IR / concept map / dep map.
