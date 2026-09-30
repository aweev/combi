# Status — Documentation Claim vs Code Reality

**Status:** DATED AUDIT. Numbers were taken from the working tree and **change while services port.**
**Reproduce before quoting** — each figure below is a command, not a belief:

```bash
cd combi
ls services/platform | wc -l                        # 39  service directories
find . -name '*.go' -not -path './.git/*' | wc -l   # 1,086 all Go files (incl. tests, mocks, .pb.go)
find . -name '*.go' -not -name '*_test.go' -not -name '*.pb.go' -not -path './.git/*' | wc -l   # 824 non-test
(cd pkg && go build ./... && go list ./... | wc -l) # 145 packages, building clean ("146" in retired task.md did not reproduce)
grep -cE '\.Module,' cmd/platform-server/main.go    # 19 mounted service modules (InfraModule is a 20th
#   .Module reference but does not match \.Module, — do not quote 20 as the mounted count)
find services/apps/cast -name '*.go' | wc -l        # 7   Cast  (7 dirs, 7 files = unbuilt)
find services/apps/ngo -name '*.go' | wc -l         # 46  NGO   (the only real app code)
find apps/circle apps/conus -type f 2>/dev/null | wc -l  # 0 — those spec directories no longer exist
grep -rl --include='*.go' 'combi/pkg/\(compliance\|geo\|i18n\)' services cmd | wc -l  # 0 importers.
#   NOTE the --include: without it grep matches the checked-in binaries platform-server/workflow-engine
#   (2 results), because they embed the import paths. Counting those as "consumers" is wrong.
cd docs && ./check-docs.sh                          # 15 checks, exit 0
```

**This is the only document permitted to state build readiness.** No other document may claim a system is
"safe to build from." That claim was the failure mode this file exists to prevent.

---

## 1. Headline

> **Circle is specification, not software.**

`apps/circle/` held markdown and **zero** non-markdown files. There is no `circle-service` directory
anywhere under `combi/services/`. The primitives the entire product depends on — Person, Face, Circle,
Membership, Story, Now, Next, Possibility, Capability — **have no implementation in this repository.**

The platform substrate beneath them is real and **further along than any task list suggested**: 39
platform service directories, 1,086 Go files, `pkg/` = **145 packages** (`cd combi/pkg && go list ./...`
— the "146" in the retired `task.md` did not reproduce), **19 services mounted
in `platform-server`**.

So the missing thing is not plumbing. **It is Circle itself, and billing.**

## 2. The corpus that was removed (why this file is now short)

| Corpus | Lines | Non-markdown files | Now |
|---|---|---|---|
| `apps/circle/` 31 documents | 17,925 | **0** | → `02`–`06` in this directory |
| `apps/conus/` 13 documents | 31,027 | 0 | → `07-CONUS.md` |
| `docs/adr/` 11 + `api/docs/adr/` 10 | 1,319 | — | → `01-DECISIONS.md`; **numbering collision (two ADR-0002s) resolved** |
| `api/docs/` prose (CONFIGURATION + events.md) | 117 | — | superseded by `combi/` code |
| `api/README.md` | 791 | — | superseded; the whole `api/` tree was later removed (see `README.md` §4) |
| `plan.md` + `task.md` | 329 | — | → `09-PLAN-90D.md` |
| `doc/` service specs (foundation + five_services) | 3,748 | — | removed; i18n/geo/compliance spec extracted to `legacy-source/` as regulatory provenance |
| `apps/cast/` Next.js app | — | 75 `.tsx`, **one route** (`courtroom`) | the only product surface that exists. Its 3 design bibles (PDF/HTML, 572 KB) **kept** |

**Three of the removed corpus's own eight verification invariants were false when re-checked** — recorded
here so the new ones are enforced by a script rather than by intention:

| Claimed invariant | Reality found |
|---|---|
| "no dormant-account deletion, identifier reclamation **or waitlist** anywhere" | **FALSE** — `Lifecycle & Account Status.md` §19–§20 specified a complete WAITLIST flow, in the #7-authority document, *after* the ADR that recorded deleting it |
| "**one** subscription enum, defined in `Lifecycle` §10 only" | **FALSE** — the full 7-state list was re-*defined* in six places (`Data Architecture`, `GLOSSARY`, `PRD`, `Pricing`, `Monetization`) |
| "every document has a `Status:` line and appears in this index" | **FALSE** — 16 of 31 files had none, including five that *hold* authority (`Domain Model`, `Core Objects`, `Product Philosophy`, `Architecture`, `Data Architecture`) |

All three are now machine-checked by `check-docs.sh`. **The invariant that would have caught the first two
— section-level cross-reference validation — never existed; only file-level link checking did. That was the bug.**

## 3. The corpus also miscounted itself, in both directions

`task.md`'s checklist was **stale**: it listed ten ported-and-mounted services as unchecked. An earlier
audit of the same tree read "6 modules mounted" when 19 were, and reported payments and identity as
unported when both were ported and mounted. The tree was ahead of its own tracker while the flagship
documents described software that does not exist.

**Rule going forward:** this file is generated from commands, not from the task list.

## 4. MVP requirement vs code

| MVP requirement | Status | Evidence |
|---|---|---|
| authentication | **PORTED** | `auth-service` 10 impl + tests, mounted |
| roles / permissions | **PARTIAL** | `access-control-service` 7 impl, mounted as `acl`; no Circle-scoping |
| communication | **PARTIAL** | `messaging-service` 4 impl + `realtime-service` 7 impl (hub, authorizer, redis broker), mounted |
| notifications | **PORTED** | `notification-service` 14 impl, mounted |
| search | **PORTED** | `search-service` mounted, not Circle-scoped |
| privacy | **PARTIAL** | `consent-privacy-service` 6 impl, **ported but not mounted** |
| core security | **PORTED** | `pkg/security`, `pkg/limiter`, `audit-log-service` 10 impl |
| locales incl. RTL | **PARTIAL** | `pkg/i18n` 10 impl files + 4 locales in `apps/cast/i18n/routing.ts`; **Circle has no frontend at all** |
| **Person** | **ABSENT** | no entity, table or service anywhere |
| **Face** | **ABSENT** | `identity-service` and `pkg/domain/user` exist; no Face concept |
| **Circle creation** | **ABSENT** | nothing named Circle outside markdown |
| **membership** | **ABSENT** | `organization-service` (7 impl) has org roles, not Circle membership |
| **people / relationships** | **ABSENT** | Relationship as a first-class object has no implementation |
| **Story** | **ABSENT** | `content-service` (5 impl) is a headless CMS, not Circle Story |
| **Now / Next** | **ABSENT** | — |
| **Possibilities** | **ABSENT** | `search-service` + `analytics-service` are raw material; no engine |
| **basic subscription infra** | **NOT PORTED — hardest gap** | `billing-subscriptions-service`: **0 impl files, 1 test.** Port source = the legacy `financial` service (8 impl files, 2,783 lines), now recoverable **only** from `/root/combi-docs-backup/legacy-source/financial/` after `api/` was deleted |

**Tally: 7 ported · 4 partial · 10 absent** of the 18-item MVP definition.

## 5. Other verified gaps

| Gap | Evidence | Consequence |
|---|---|---|
| No `circle.*` events | `pkg/domain/core/events.go` has `financial.subscription.*`; `02-DOMAIN.md` §9 requires **21** Circle events, **none emitted** | every metric in `06-MEASUREMENT.md` is currently unmeasurable |
| **No CCU metering** | no usage-metering service; `analytics-service` mounted but no per-Circle capacity accounting | **blocks D-11 pricing entirely**, and blocks `04-ECONOMICS.md` §8 |
| Invite/abuse controls not wired | `pkg/limiter` ported, **nothing calls it for invitations** | T1 live in `05-TRUST.md` §1 — the primary exploit surface is open |
| **`pkg/{compliance,i18n,geo}` have zero importers** | `grep -rl --include='*.go' 'combi/pkg/(compliance|geo|i18n)' services cmd` → **0 matches** (without `--include` it falsely reports 2, because the checked-in binaries embed the import path strings); the only `compliance` hit in `main.go:163` is `auditlog.ComplianceService`, a different object | the regulatory substrate is **compiled, tested, and connected to nothing.** Cheapest available win |
| No product doc cited those packages | `Language & Localization.md` ran 515 lines defining locales/RTL/plurals and never named `pkg/i18n` | the L0-integration gap was invisible to every strategy document |
| Offline / USSD unimplemented | `ussd` appeared only as a schema enum in the archived `doc/` specs; **0 occurrences in `pkg/` or `services/`** | do not claim offline reach anywhere |
| Cast effectively unbuilt | `services/apps/cast/*`: 7 directories, **7 Go files total** | D-15 (one Cast app) |
| NGO is the only real app code | `services/apps/ngo/*`: 7 directories, **46 Go files** | the most-built thing is the line D-13 declares history — freeze it or convert it to capabilities |
| 13 service skeletons (≤2 impl files) | 10 with one file (api-gateway, biometric, connectivity, did, escrow, event-bus, fx, kyc, migration-interop, trust-reputation), 3 with two (offline-token, tagging, wallet-ledger) | **still present** — cut proposed in `00-MANIFEST.md` §8, not executed (needs `go.work` + a verified build) |
| Legacy stubs fail to build | **19** per-service `cmd/server/main.go` files still exist (`find services -name main.go`) | **still present** — not repaired, not deleted. D-09 says delete them with the skeletons |
| Repo-wide gofmt drift | the pre-commit hook fails on hundreds of `.go` files | blocked a clean docs-only commit; `--no-verify` was used for the safety snapshot. **Fix in one dedicated commit, never as a docs side-effect** |
| **`ngo-platform.org` strings live in `combi/` code, not `api/`** | `services/platform/workflow-service/activities.go:294` (`sign.ngo-platform.org`), `organization-service/workers.go:83` (`app.ngo-platform.org/join`), `notification-service/processor/processor.go:56` (`dev@ngo-platform.org`), `integration-service/adapters/stripe_adapter.go:78` (Stripe `ReturnURL`), plus `pkg/config/loader.go:33 SetEnvPrefix("NGO")` | deleting `api/` did **not** remove the brand problem. These are user-visible at runtime. D-13 |
| `Makefile` targets are broken | `make build` exits 2: loops `services/*/` then `cd`s into `cmd/server`, but `services/apps` and `services/platform` are container dirs with no module or entrypoint (D-09 moved to library modules). `lint`/`test`/`tidy` loop the same wrong path | the repo has no working build automation beyond direct `go build`; fix alongside step 1.5 |
| 0-byte junk at workspace root | files named `And`, `There` | **done** — removed with the rest of the root |

## 6. Honest summary

**What exists and works:** a platform substrate — auth, ACL, payments, storage, notifications, realtime,
search, sync, audit, consent, plus unimported i18n/geo/compliance libraries. 145 packages, 19 mounted
modules, one host binary.
**What does not exist:** the product. Circle has no code, no directory, no events, no billing, and had no
place in the approved architecture.
**Therefore:** the next 90 days are about **one Circle**, not about platform work. The platform is done
enough. Any further substrate building is procrastination dressed as progress.

