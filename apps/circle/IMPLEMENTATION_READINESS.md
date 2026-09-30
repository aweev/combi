# Circle Implementation Readiness

**Status:** AGREED (audit), captured from the working tree on 2026-09-30.
**Purpose:** map each `PRD` §101 MVP requirement to code that actually exists, so the
specification set cannot be mistaken for progress.
**Reproduce:** walk `services/platform/*`, parse the `.Module,` mounts in
`cmd/platform-server/main.go`, count non-test `.go` files.
**Caution:** another process was porting services *during* this documentation sync
(mounted modules went 6 → 18 → 19 within one session). Re-run before quoting.

---

## 1. Headline

**Circle is specification, not software.** `apps/circle/` holds markdown documents and
**zero** non-markdown files. There is no `circle-service` directory anywhere under
`combi/services/`. The primitives the entire product depends on — Person, Face,
Circle, Membership, Story, Now, Next, Possibility, Capability — have **no
implementation in this repository**.

The platform substrate beneath them is real and further along than the task list
suggests: 39 platform service directories, ~1,079 Go files in `combi/`, and `pkg/`
reporting 146 clean packages (`task.md` Phase 0).

## 2. Corrections to statements made earlier in this engagement

The first analysis relied on `task.md` plus an early read of `main.go`. Both were
misleading, in opposite directions:

| Earlier claim | Verified reality |
|---|---|
| "only 6 modules mounted" | **19** mounted: acl, analytics, auditlog, auth, consentprivacy, feature, identity, integration, localization, messaging, monitoringsvc, notification, organization, payments, realtime, search, secretsconfig, storagesvc, syncsvc |
| "payments unported, 11 Go files total" | `payments-service` has **7 impl + 4 test and IS mounted**. The 11 conflated payments with billing |
| "identity not ported" | `identity-service`: 6 impl + 3 test, **mounted** |
| "storage / realtime / search / organization / analytics pending" | all ported **and mounted** (storage largest at 20 impl files) |
| `task.md` Phase 3 checklist | **stale** — lists ten ported+mounted services as unchecked |

So the substrate is **ahead of its own task list**. What is missing is not plumbing;
it is **Circle itself** and **billing**.

## 3. MVP requirement vs code (`PRD` §101)

| Requirement | Status | Evidence |
|---|---|---|
| authentication | **PORTED** | `auth-service` 10 impl + 7 test, mounted |
| Person | **ABSENT** | no entity, table or service anywhere |
| Face | **ABSENT** | `identity-service` and `pkg/domain/user` exist; no Face concept |
| Circle creation | **ABSENT** | nothing named Circle outside markdown |
| membership | **ABSENT** | `organization-service` (7 impl) has org roles, not Circle membership |
| roles / permissions | **PARTIAL** | `access-control-service` 7 impl + 6 test, mounted as `acl` |
| people / relationships | **ABSENT** | Relationship as a first-class object has no implementation |
| Story | **ABSENT** | `content-service` (5 impl) is a headless CMS, not Circle Story |
| Now | **ABSENT** | — |
| Next | **ABSENT** | — |
| Possibilities | **ABSENT** | `search-service` + `analytics-service` are the nearest raw material; no Possibility engine |
| communication | **PARTIAL** | `messaging-service` (4 impl) + `realtime-service` (7 impl: hub, authorizer, redis broker), both mounted |
| notifications | **PORTED** | `notification-service` 14 impl + 3 test, mounted |
| search | **PORTED** | `search-service` mounted, not yet Circle-scoped |
| privacy | **PARTIAL** | `consent-privacy-service` 6 impl + 3 test, mounted; no contextual-permission model |
| 4 locales incl. RTL | **PARTIAL** | `localization-service` (3 impl) mounted; Cast ships `en-NG, pcm-NG, fr-CM, ha-NG-Arab` in `apps/cast/i18n/routing.ts`; **Circle has no frontend at all** |
| basic subscription infrastructure | **NOT PORTED — hardest gap** | `billing-subscriptions-service`: **0 impl files, 1 test file**. Material exists (`pkg/domain/financial` Subscription/Payment models; `api/internal/services/financial` is production-grade) but the module is empty |
| core security | **PORTED** | `pkg/security`, `pkg/limiter`, `audit-log-service` (10 impl + 7 test) |

**Tally: 4 ported · 4 partial · 10 absent** of 18 MVP bullets.


## 4. Other verified gaps

| Gap | Evidence |
|---|---|
| `plan.md` has no category for Circle | §40 lists `platform/`, `apps/cast/`, `apps/ngo/` only — the flagship cannot be placed by the approved migration plan |
| No `circle.*` events | `pkg/domain/core/events.go` has `financial.subscription.*`; `Measurement.md` §5 requires 17 new Circle events, none emitted |
| Invite/abuse controls not wired | `pkg/limiter` ported, nothing calls it for invitations (`Trust & Abuse Operations.md` §8) |
| No CCU metering | no usage-metering service; `analytics-service` mounted but not per-Circle capacity accounting — **blocks ADR-0003 pricing entirely** |
| Cast effectively unbuilt | `services/apps/cast/*`: 7 directories, **7 Go files total** |
| NGO is the only real app code | `services/apps/ngo/*`: 7 directories, **46 Go files** — app-level work went to a product line that has no spec set here |
| Legacy stubs fail to build | `task.md:19` — ~20 per-service `cmd/server/main.go` stubs broken (pre-existing; superseded by host-mounted modules) |
| No git history before this sync | `combi/.git` had **no commits**; baseline is `25126107` |
| Working tree has concurrent non-doc edits | `cmd/platform-server/{main.go,go.mod,go.sum}`, `pkg/fxcore/module.go`, `analytics-service`, `search-service/fx.go`, plus rebuilt binaries — **deliberately excluded** from the docs commits |

## 5. Smallest credible path to a billable Circle

Ordered by dependency, not by desire:

1. **`circle-service`** — Person, Face, Circle, Membership, Role bindings, the four
   archetypes, Branch depth 1 (ADR-0008). Mount it in `platform-server`.
2. **Story / Now / Next / Possibility** as projections over Circle activity
   (`Domain Model` §12), not as new products.
3. **Port billing** — `api/internal/services/financial` → the empty
   `billing-subscriptions-service`, using ADR-0001's state machine as the enum and
   `Pricing` §33's pricing-versioning requirement.
4. **CCU metering** (ADR-0003) — no price can be invoiced without it.
5. **Invite + rate limits** wired to `pkg/limiter` (`Trust & Abuse` §8) **before** any
   growth loop is opened; invitations are the primary exploit surface.
6. **Consent / minor model** (`Trust & Abuse` §4) before Family ships — Family is a
   launch archetype and minors are expected in it.
7. **One capability end-to-end** through `Capability Contract.md`. Recommendation:
   Vault **Level 1 (record only)** — Contribution + Obligation. It is the Fellowship
   wedge, needs no licence, and proves the contract on a real path.

Steps 1–3 are the minimum for "basic subscription infrastructure" to stop being an MVP
checkbox that no code satisfies.

## 6. What this document is not

It is not a report on the documentation sync (that is `DOC_INDEX.md` plus
`../../docs/adr/`). It is not a stable measurement: the counts above were taken from a
tree another process was actively editing, so re-run the reproduction step in the
header before quoting any number from it.
