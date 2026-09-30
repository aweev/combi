# Combi Documentation — Index

**Status:** This file replaces `combi/apps/circle/DOC_INDEX.md` and all 19 legacy ADR files.
**Rule:** there are eleven documents. If a twelfth is proposed, one of these must absorb it or be deleted.

---

## 1. Read in this order

| # | Document | Audience | Owns |
|---|---|---|---|
| 0 | [00-MANIFEST.md](00-MANIFEST.md) | everyone | thesis, architecture, wedge, metrics, kill list |
| 1 | [01-DECISIONS.md](01-DECISIONS.md) | everyone | every open and settled decision, with status |
| 2 | [02-DOMAIN.md](02-DOMAIN.md) | engineering | objects, state machines, permissions, events — **the build contract** |
| 3 | [03-CAPABILITY.md](03-CAPABILITY.md) | engineering | how a capability installs into a Circle |
| 4 | [04-ECONOMICS.md](04-ECONOMICS.md) | founder + finance | CCU, pricing structure, Vault levels, cost gaps |
| 5 | [05-TRUST.md](05-TRUST.md) | ops | abuse model, rate limits, minors, retention |
| 6 | [06-MEASUREMENT.md](06-MEASUREMENT.md) | product | North Star, metrics, anti-metrics, event properties |
| 7 | [07-CONUS.md](07-CONUS.md) | media team | CONUS as a Circle tenant, one engine, sponsorship |
| 8 | [08-STATUS.md](08-STATUS.md) | engineering | **doc claim vs code reality — the only build-readiness record** |
| 9 | [09-PLAN-90D.md](09-PLAN-90D.md) | everyone | the next 90 days, dependency-ordered |

Reference (not strategy): `../../doc/` — the Africa OS platform specs for `pkg/i18n`, `pkg/geo`,
`pkg/compliance`. Superseded by code where they conflict; retained as regulatory-data provenance.

## 2. How conflicts are resolved

1. **One document owns one domain** (column 3 above). No other document restates its rules — it links.
2. Where a document and the code disagree, **the code wins** and `08-STATUS.md` records the gap.
3. Where two documents disagree, the lower number in §1 wins, **unless** `01-DECISIONS.md` says otherwise.
4. Money, brand and lifecycle decisions carry a status. `PROPOSED` means **not decided**, regardless of how many documents reference it.

**"Safe to build from" is not a document property.** Only `08-STATUS.md` may state build readiness.

## 3. Invariants

Each is machine-checked by `./check-docs.sh`, which prints one line per rule. A rule that cannot be
checked is not a rule, it is a rumour.

1. Only the approved eleven documents exist here. Adding one requires deleting one.
2. Every document declares `**Status:**` within its first 12 lines.
3. No document self-declares `AGREED` / "safe to build from" — readiness belongs to `08-STATUS.md` only.
4. Each state enum (subscription, account, data, membership, capability, identifier) is **defined in
   `02-DOMAIN.md` §8 and nowhere else**.
5. Four archetypes only: FAMILY, FELLOWSHIP, BUSINESS, COMMUNITY (D-01).
6. No waitlist, long-list or reactivation queue outside the document that records its deletion (D-10).
7. Nothing is deleted or reclaimed for dormancy, lapse or non-payment (D-10).
8. No price escalates with platform size (D-12).
9. No unsourced market sizing — no TAM, no "market size", no population figures.
10. Every relative `.md` link resolves.
11. Banned vocabulary (feed, followers, likes, streak) appears only inside a paragraph that forbids it.

Run: `cd combi/docs && ./check-docs.sh` (exit 0 = clean). Checks 6–8 and 11 are **paragraph-level**:
a phrase is legal only when the same paragraph negates it, because prose wraps and a line-based test
either misses the negation or fires on the tombstone itself.

## 4. What was removed, and why

**Measured against git snapshot `d5df562c`** (so the figure cannot drift as these files are edited).
The previous corpus held **54,465 lines of markdown**, replaced by the eleven documents above
(~1,700 lines) — a **97% cut**:

| Removed | Lines | Files |
|---|---|---|
| `apps/circle/` product specs | 17,925 | 31 |
| `apps/conus/` product specs | 31,027 | 13 |
| `combi/docs/adr/` | 787 | 11 |
| `api/docs/adr/` (numbering collided with ours — two files numbered 0002) | 532 | 10 |
| `api/docs/` prose (CONFIGURATION, events) | 117 | 2 |
| `plan.md` + `task.md` | 329 | 2 |
| `doc/` service specs (the i18n/geo/compliance spec is kept) | 3,748 | 2 |

Against **824 non-test Go files**, that was ~50 prose lines per source file — and `apps/circle/` held
17,925 of those lines with **zero** implementation files beside them.

Kept intentionally: `api/README.md` (791 lines, describes the still-running legacy `api/` system),
`api/docs/architecture/service-portability-standard.md` (cited by `03-CAPABILITY.md` §2), the `api/docs/`
runbooks for the legacy host, and `doc/africa_os_i18n_geo_compliance_spec.md` (provenance for the
regulatory data now in `pkg/compliance`).

**Recovery:** `combi/` items are in git (`git show d5df562c:<path>`). Items outside `combi/` are **not
versioned** — they are archived at `/root/combi-docs-backup/untracked-2026-09-30.tar.gz`.

The removed set also failed **three of its own eight** self-declared invariants (a waitlist survived the
decision that deleted it; the subscription enum was defined six times; sixteen files had no `Status:`
line, five of them top-authority). Those contradictions are resolved here rather than re-argued, and are
now enforced by `check-docs.sh` instead of by intention.
