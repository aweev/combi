# 90-Day Plan — One Working Circle

**Status:** ACTIVE. Replaces `plan.md` (the 6-phase api→combi migration) and `task.md` (its stale checklist).
**Objective:** not "ship a platform." **Prove that a real group will use Circle to record money and
obligations, and that the record is the reason they stay.**
**Definition of done at day 90:** MCT measured on a real cohort **and** ≥ 1 Circle where an obligation was
recorded *and* settled. If both fail, change the wedge — keep the thesis (Manifest §10).

---

## Weeks 1–2 — Decide, then delete

| # | Task | Status | Why it is first |
|---|---|---|---|
| 1.1 | **Sign the five decisions** in `01-DECISIONS.md` (D-11, D-12, D-10, D-13, ratify D-07) | ☐ **founder** | everything downstream is blocked on money + broker. Four were left "Proposed" while 24 of 26 documents were stamped "safe to build from" |
| 1.2 | Resolve **D-16 item 1** ("active seat") and **item 2** (CCU weights) | ☐ **founder** | invoicing is uncomputable without them |
| 1.3 | `check-docs.sh` + `.github/workflows/docs.yml` | ☑ **done** | the old corpus failed 3 of its own 8 invariants; `workflows/` was empty, so enforcement had no mechanism. 15 checks now run on every docs change |
| 1.4 | Remove the 54,465-line legacy corpus (44 product docs, 21 ADR files, `plan.md`, `task.md`, `api/docs` prose, 2 `doc/` specs) | ☑ **done** | the doc meta-layer was a tax levied by document volume. Recovery: git `d5df562c` + `/root/combi-docs-backup/untracked-2026-09-30.tar.gz` |
| 1.5 | Delete the 13 service skeletons (≤2 impl files) and the **19** legacy `cmd/server/main.go` stubs, editing `go.work` | ☐ | stops paying per-directory CI and dependency cost for phantom services. **Not repaired, not deleted — still present.** Requires a verified `go build`, so it is a code change, not a docs change (D-09) |
| 1.6 | One dedicated commit: `gofmt -w .` | ☐ | the pre-commit hook currently fails on hundreds of files; never bundle it with a feature |
| 1.7 | `go run ./cmd/service-init -tier platform circle` → create `services/apps/circle/` | ☐ | the flagship had no home in the approved architecture |

## Weeks 3–8 — Build the Circle

**Dependency order, not desire order.** Each step compiles and passes tests before the next starts.

| # | Task | Reuse | Notes |
|---|---|---|---|
| 2.1 | `circle-service`: Person, Account, Face (3 types), Circle, Membership, Role bindings, 4 archetypes, Branch depth 1 | `pkg/domain`, `pkg/persistence`, `access-control-service` | mount in `platform-server`. **This is the whole product thesis or it is nothing** |
| 2.2 | Relationship as a first-class object + Affiliation | `pkg/domain` | no new primitive needed (D-03) |
| 2.3 | Story / Now / Next as **projections over Activity** | `analytics-service`, `search-service` | append-only history, derived projections (`02-DOMAIN.md` §12). Not new products |
| 2.4 | **Port billing**: `api/internal/services/financial` (8 impl files) → the empty `billing-subscriptions-service` | `pkg/domain/financial` models | use the D-06 state machine as the enum. Closes the "hardest gap" |
| 2.5 | **CCU metering v0** — three dimensions only: storage, seats, API calls | `analytics-service`, `pkg/cache` | one system, four jobs (`04-ECONOMICS.md` §8). No price can be invoiced without it |
| 2.6 | **`circle.*` events** — emit the `02-DOMAIN.md` §9 set through `events.Emit` | `pkg/events` (NATS, per D-07) | without this, every metric in `06-MEASUREMENT.md` is unmeasurable |
| 2.7 | **`pkg/limiter` on invites + Circle creation** | already ported | **before opening any growth loop.** Invitations are the primary exploit surface (T1) |
| 2.8 | **Consent + minors**: mount `consent-privacy-service`, add `age_band` + guardian consent | already ported, unmounted | **before any Family-facing surface** |
| 2.9 | **Wire `pkg/compliance` + `pkg/i18n` + `pkg/geo` into circle-service** | 22 impl files, currently **zero importers** | cheapest strategic win available: country rules and four locales are paid for and idle |

## Weeks 9–10 — The wedge

| # | Task | Scope discipline |
|---|---|---|
| 3.1 | **Vault Level 1**: Contribution + Obligation + Settlement + Evidence, installed as the first capability through `03-CAPABILITY.md` | record only. **No Level 2, no custody, no payment provider** |
| 3.2 | Prove the Capability Contract on a real path: manifest declares `ccu_dimensions`, disable-safe, no forked identity | the six §7 answers written in the PR |
| 3.3 | Export: machine-readable bundle including capability-owned records | portability is what makes trust credible rather than coercive |

## Weeks 11–12 — Real Circles

| # | Task | Target |
|---|---|---|
| 4.1 | Recruit **5–10 real Fellowship Circles** by hand — savings groups, congregations, trade associations, alumni, sports teams | no landing page, no waitlist (D-10), no advertising |
| 4.2 | Record and reconcile their actual obligations — notebook / WhatsApp / paper → Vault | the test is whether the record changes group behaviour |
| 4.3 | Instrument the three Manifest §6 metrics; stand up dashboards 1, 3, 4 | **data loss must read zero** — if it reads non-zero, D-10 is already broken |
| 4.4 | Fill **C1, C4, C5** of the cost model from real usage and provider rate cards | gates any published price |
| 4.5 | Write the appeal workflow before any `RESTRICT` ships | `05-TRUST.md` §3 |

## Explicitly NOT in these 90 days

CONUS engines 2–8 · Cast (D-15) · advertising · marketplace · IP/licensing · Vault custody (Level 3) ·
native apps · GraphQL BFF · the 39-service topology · Possibility AI beyond rule-based v0 · Projects,
Events, Documents, Media pipeline, Governance, Automation · a second event broker · any new user-facing
noun · any published price.

## Sequencing rule that ends the four-corpora problem

**Circle first as platform. CONUS as its first tenant. Cast behind the contract.**
Launching all four as siblings is what produced four document sets that never referenced each other.
CONUS may build Daily CONUS during these 90 days **only** against the `07-CONUS.md` §4 seam, storing
`circle_person_id` as nullable, and **may not create its own membership or permission model.**

## Kill / continue gates

| Signal at day 90 | Action |
|---|---|
| ≥ 1 Circle with a settled obligation **and** MCT measurable | proceed; add CONUS as tenant #2 |
| Circles created, no obligations settled | the wedge is wrong: the pain is not money. **Re-test with Family continuity — not by adding features** |
| No Circle reaches MCT at 60 days | stop building substrate; interview every user; change the archetype, not the architecture |
| CONUS has begun minting its own identity | **halt CONUS.** That is the D-14 failure mode made real |
| Any data loss from dormancy or lapse | treat as a **P0 policy breach**, not a bug (D-10) |

## Standing rules

1. **No new document.** Add a section, or delete a paragraph elsewhere.
2. **No new user-facing noun** without editing D-05.
3. **No state enum defined outside `02-DOMAIN.md` §8.** Link, never restate.
4. **No number in a pricing or market document without a cited source.** Blank beats guess.
5. **Re-run `08-STATUS.md`'s commands before quoting any count.** The tree moves while services port.
6. **`check-docs.sh` must pass in CI** before a docs PR merges.

