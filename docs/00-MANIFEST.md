# Combi Manifest

**Status:** PROPOSED — requires founder sign-off on the five decisions in `01-DECISIONS.md`.
**Supersedes:** the entire `apps/circle/` + `apps/conus/` document corpora and `plan.md`.
**Length limit:** this document stays under 300 lines. If it grows, cut it; do not split it.

---

## 1. Thesis

> **Combi is the trust substrate for collective life: a durable record of who belongs where,
> what they committed, what they contributed, and what it produced — portable across every
> product, institution and third party that needs to know who can be trusted with what.**

People already organise their lives in groups — families, savings circles, congregations, trade
associations, project teams, neighbourhoods. Those groups hold real value: obligations, contributions,
decisions, history, standing. Almost all of it is recorded in notebooks, screenshots and chat threads
that are unsearchable, unrecoverable and non-portable.

Combi's product is not another place to chat. It is the **authoritative record of a group's commitments
and history**, plus the permission model that makes that record safe to share selectively.

**Position, stated plainly:** we are the system of record for small groups. Everything else — feeds,
messaging, media, marketplaces — is someone else's business, and we integrate with it rather than
competing with it.

## 2. Why now

- The substrate that would make this possible is **already largely built and entirely unused.**
  `combi/pkg/` is 145 packages that build clean (`go build ./...`), including an event bus, RBAC, audit-log with crypto-shredding,
  consent/privacy, storage, sync and payments — 824 non-test Go files, 19 services mounted in one host.
  No product consumes it.
- `pkg/compliance`, `pkg/i18n`, `pkg/geo` implement **per-country KYC tiers with named regulatory
  citations** (COBAC R-2019/01 Art. 14/15), transaction limits, and sanctions sources (UN SC, OFAC SDN,
  EU FSF, COBAC AML, CBN BVL, CBK CRB) across eight countries, plus four locales including RTL.
  **Nothing imports them.** That is sunk cost awaiting its first customer, not a roadmap item.
- The record-only Vault needs **no licence** (see `04-ECONOMICS.md` §3). Most of the compliance surface
  that stops fintech entrants does not apply to us at V1.

## 3. Architecture: four layers, two of which we build

```text
L0  TRUST SUBSTRATE            ← the company. Owns this absolutely.
    Person · Face · Circle · Membership · Role · Permission
    Activity · Story · Now · Next · Commitment · Obligation · Transaction
    CCU metering (capacity + billing + quota + tenant isolation, one system)
    pkg/{compliance, i18n, geo, security, audit, events, sync}

L1  CONTRACT                   ← what makes us a platform rather than an app
    Capability Contract (03-CAPABILITY.md): install/uninstall, declared CCU
    consumption, no forked identity, no forked money, no new nav destination
    One event namespace: circle.<object>.<past-tense-action>

L2  WEDGE PRODUCTS             ← maximum two live at a time
    #1  Fellowship Vault Level 1 (record-only money ledger)
    #2  CONUS, as a tenant — never a sibling

L3  ECOSYSTEM                  ← only once L0 has an external consumer
    third-party capabilities · compliance-grade attestations · continuity metrics
```

**Build L0 and L1. Ship one L2. Do not start L3 until a third party signs.**

## 4. The wedge: Fellowship Vault Level 1

A Fellowship Circle (savings group, congregation, trade association, sports team, alumni group)
records who owes what, who paid, and what the group decided — **without holding money.**

| Test | Answer |
|---|---|
| Existing behaviour with existing pain? | Yes. Rotating savings, contributions and levies already happen and are already recorded badly. |
| Regulatory exposure? | None at Level 1. Record only; custody (Level 3) is out of scope by decision. |
| Who pays, and can they? | The Circle's Billing Account, per CCU — organisations, not individuals. |
| Does it build the graph? | Yes, unavoidably. Money forces members, roles, obligations and history to be named correctly. |
| Needs AI / media / ads / events / native apps? | No. |
| Proves the Capability Contract? | Yes — Vault is the first capability installed through it. |
| Unlocks the blocked economics? | Yes — real usage to meter is the precondition for any price. |

**Fellowship is the launch archetype** because it is the least sentimentally-loaded and most
legally-clean of the four, and because a group with money moving is a group that needs a record.


## 5. Growth model

One loop, and it is not a feed:

```text
A group records an obligation
  → the record makes the group's money visibly fairer
  → members invite the missing people (the invite is the ONLY growth surface)
  → the group's history becomes the reason staying is cheaper than switching
  → more circles of the same kind arrive by referral, not by advertising
```

**Switching cost is the moat, and it is earned the honest way: continuity.** A Circle that has recorded
three years of contributions does not leave for a competitor with a better feed. This is the one
defensible answer to "why will this last."

A second loop is added only after L2 #1 works: a CONUS participation event can write a Circle Story,
and a Circle commitment can surface a CONUS opportunity. **Media acquires, substrate retains.** Until
the substrate exists that loop is a slogan, and CONUS will simply build its own identity system — at
which point the argument for one company dies. Tracked as risk D-07 in `01-DECISIONS.md`.

## 6. Metrics: exactly three

| Metric | Definition | Why |
|---|---|---|
| **North Star** | Continuing Circles per 1,000 people — Circles meeting MCT at day 60, per 1,000 members | a rate, so growth cannot hide dysfunction |
| **Wedge proof** | Recorded obligations settled per Circle per month | proves the ledger reflects real money behaviour, not data entry |
| **Solvency** | CCU consumed vs. measured cost per Circle per month | fills the cost model that gates every price |

**MCT (Meaningful Continuity Threshold)** — a Circle counts at day 60 when **all** are true:
1. ≥ 2 members with `ACTIVE` membership;
2. ≥ 3 Story entries created by ≥ 2 distinct members;
3. ≥ 1 `Next` item created or updated in the last 30 days;
4. ≥ 1 completed interaction attributable to the loop (commitment fulfilled, contribution recorded,
   reply to a `Now` item, or action on a Possibility).

Criterion 4 is the loop test: a Circle that only accumulates records but never acts has not closed.

Anti-metrics, banned as targets: DAU/MAU ratio, time-in-app, notification volume or open rate as a goal,
posts per user, follower/connection counts, streaks that reward opening the app without doing anything.

> **Rule: if a metric can be improved by any anti-metric, the metric is wrong. Fix the metric, not the behaviour.**

## 7. What we refuse to become

Load-bearing constraints, not style preferences. Each protects the moat in §5.

1. **No feed, no followers, no likes.** Circle is a record of a group, not an audience.
2. **No advertising and no sale of personal data** in Circle's economy. Margin comes from payer mix.
3. **Never bill a person for belonging.** Bill the Billing Account that owns the Circle.
4. **Never delete or reclaim anything for non-payment or dormancy** (D-03).
5. **No global trust score, no numerical social credit visible to other users.** Verification is scoped per Circle.
6. **Possibility is not prediction.** Surface with provenance; the person decides.
7. **A capability never mints its own login, account type, permission system or nav item.**
8. **Nesting depth 1.** Branch or affiliate; do not build an org chart or an audience layer.
9. **Seven user-facing nouns, forever** (Circle, Face, Story, Now, Next, Vault, Possibility). New nouns require a decision.

## 8. Kill list — effective now

| Cut | Reason |
|---|---|
| **NGO as a product line** | The 7 `apps/ngo/*` services (46 Go files — the only real app code) land as *capabilities* for Community/Business Circles, or are frozen. No spec set, no roadmap, no brand. |
| **Cast as seven services** | 7 directories, 7 Go files total. Becomes **one** capability or one app. |
| **13 service skeletons holding ≤2 impl files** | 10 with exactly one file (api-gateway, biometric, connectivity, did, escrow, event-bus, fx, kyc, migration-interop, trust-reputation) and 3 with two (offline-token, tagging, wallet-ledger). `api-gateway-service` is the signature artifact of the topology abandoned in favour of 3 binaries. (`billing-subscriptions-service` has **0** impl files but is kept: it is the port target.) **Proposed, not yet executed** — see §8 status note. |
| **CONUS engines 2–8** | Short, Founders, Pitch, Challenge, Live, Fan, Local — all after Daily CONUS proves return behaviour. |
| **54,465 lines of pre-signal spec** (measured in `docs/README.md` §4) | 13 CONUS documents (31,027 lines) + 31 Circle documents (17,925) + 21 ADR files + `api/docs` prose + `plan.md`/`task.md` + 2 `doc/` service specs. Writing a 4,066-line Technical Architecture and a 2,711-line QA spec before one user exists violated CONUS's own cost discipline: *"spend to prove behavior."* **Executed** — archived in git `d5df562c` and the tar backup. |
| **Offline/USSD reach claims** | Specified only in the now-archived `doc/` specs; implemented **zero** times in `pkg/` or `services/`. Absent from all pitch material until code exists. |
| **The doc meta-layer** | GLOSSARY / DOC_INDEX / 21 ADRs / 4 superseded stubs existed only to keep documents consistent with documents. That tax is now a shell script. **Executed.** |

**Status of the code cuts** (service skeletons, legacy `main.go` stubs, NGO/Cast directories): these are
**proposed, not executed.** They touch `go.work` and require a verified build, so they belong in
`09-PLAN-90D.md` week 1, not in a documentation change. See `08-STATUS.md` §5 for what is actually deleted
versus merely decided.

## 9. What this document does not claim

- **No market size.** There is no TAM figure anywhere in this corpus because there is no measured one. A number without a source is a guess wearing a decimal point.
- **No revenue projection.** `04-ECONOMICS.md` lists eight required cost inputs; none are measured. Structure is settled, arithmetic is not.
- **No launch date.** `09-PLAN-90D.md` commits to 90 days of learning, not to a market.
- **No offline/USSD/DID/ZK capability.** Spec lines, not code.
- **No Circle software today.** `08-STATUS.md` §1 states it plainly: *the flagship is specification, not software.* This manifest is the plan to change that, not evidence that it has.

## 10. The standard this document is held to

> A foundational layer is not declared. **It is the thing other products cannot afford to re-implement.**

Every 90-day cycle must end able to answer: *has any product — ours or someone else's — now found it
cheaper to use Circle's primitives than to build its own?* If the answer is no for two consecutive
cycles, the wedge is wrong. Change the wedge; keep the thesis.

