# Fuzz corpus

Seed inputs for any future fuzz / property-based / differential test runners.

- `corpus/` — small JSON IR examples. The dumb agent may add to this freely
  (subject to the 5-file-per-PR cap and the path allowlist). A corpus entry is
  not authoritative; it just exercises the parser/validator surface.

Corpus entries should be:

- Small (< 4 KB).
- Self-contained (no external file references).
- Deterministic (no timestamps, random fields, environment-dependent paths).

If a corpus entry triggers a verifier crash or accepts something surprising,
that is signal — write a `reports/guard-candidates/<id>.md` instead of
modifying the verifier.
