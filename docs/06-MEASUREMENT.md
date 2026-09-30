# Measurement

**Status:** V1 METRICS SETTLED.
**Authority:** measurement only. It does not redefine product behaviour.
**Companion:** CONUS keeps its own North Star (WQP) in `07-CONUS.md` §7 — separate product, separate metric.

---

## 1. Principle

Measure product health **without turning human relationships into engagement bait**.
We do **not** optimise DAU, session length, notifications sent, or impressions.
We **do** optimise whether something meaningful was preserved, understood or completed.

## 2. North Star

> **Continuing Circles per 1,000 people** — Circles meeting the **Meaningful Continuity Threshold (MCT)**
> at day 60, per 1,000 members.

**MCT — a Circle counts when all are true at day 60 after creation:**
1. ≥ 2 members with `ACTIVE` membership;
2. ≥ 3 Story entries created by ≥ 2 distinct members;
3. ≥ 1 `Next` item created or updated in the last 30 days;
4. ≥ 1 completed interaction attributable to the loop (commitment fulfilled, contribution recorded,
   reply to a `Now` item, or action on a Possibility).

Criterion 4 is the loop test: a Circle that only accumulates records but never acts has not closed.

**Why "per 1,000 people" and not an absolute count:** an absolute number grows with sign-ups, which
rewards acquisition over health. A **rate** forces the product to make Circles work rather than merely
create them. This is deliberate and must not be "simplified" into a total.

## 3. Supporting metrics

| Tier | Metric | Definition | Guards against |
|---|---|---|---|
| Activation | Circle Creation Success | new people with a ≥2-member Circle within 7 days | sign-up churn |
| Activation | Second-Person Rate | new Circles reaching 2 active members in 14 days | empty-shell Circles |
| Continuity | Story Density | median Story entries per member at day 60 | thin records |
| Continuity | Recall Use | % of members opening Story (not Now) weekly | feed-shape drift |
| Loop | Commitment Completion | % of `Next` items closed rather than expired | intention debt |
| Loop | **Possibility Acceptance** | % of surfaced Possibilities acted on | unhelpful AI |
| **Wedge** | **Vault Activity** | % of Circles with ≥1 recorded transaction in 30 days | the wedge failing |
| **Wedge** | **Obligation Settlement Rate** | settled ÷ recorded, per Circle per month | ledger-as-data-entry |
| Retention | Circle Survival @ 180d | % of MCT Circles still MCT at day 180 | vanity launches |
| Monetization | Paid Circle Share | % of Circles on paid plans, **by archetype** | cannibalising the free core |
| Monetization | Expansion by Band | band escalations per quarter vs the rate cap | unexpected bill jumps (D-12) |
| Solvency | **CCU vs Cost** | CCU consumed per plan against real infrastructure cost | unpriced capacity (C1–C8) |

**The three bolded rows are the Manifest §6 spine.** Everything else is diagnostic.

## 4. Anti-metrics — explicitly not optimised

Banned as targets: raw DAU/MAU ratio, time-in-app, notification volume or open rate as a goal, posts per
user, connection/follower counts, and streak mechanics that reward opening the app without doing anything.

> **Rule: if a §3 metric can be improved by any of the above, the metric is wrong. Fix the metric, not the behaviour.**

## 5. Event model

Naming and required properties are defined **once**, in `02-DOMAIN.md` §9. This document consumes them.
None of the `circle.*` events are emitted today — `08-STATUS.md` §4 — so every row in §2 and §3 is
currently unmeasurable. **Instrumentation is not a later phase; it is part of building the object.**

## 6. Dashboards (five, and only five)

1. **Continuity health** — MCT Circles per 1,000 people, weekly, split by archetype.
2. **Loop closure** — Possibility surfaced → acted → Story entry created.
3. **Lapse funnel** — `PAST_DUE → GRACE → RESTRICTED → INACTIVE → CLOSED`. This is the instrument that
   proves D-10 is holding: **data loss must read zero.** If it reads non-zero, the policy has already been broken.
4. **Capacity vs cost** — CCU consumed per plan against real infrastructure cost. Populates `04-ECONOMICS.md` §5.
5. **Locale parity** — MCT rate per locale, to catch `pcm-NG` or `ha-NG-Arab` users receiving a weaker
   experience. Four locales is a promise; parity is how we know whether we kept it.

One owner per dashboard. If a dashboard cannot be read weekly by one person, it is deleted.

## 7. Analytics retention

Draft; **not legally reviewed.** See `05-TRUST.md` §9.

| Data class | Retention | Basis |
|---|---|---|
| Aggregate daily rollups | Indefinite | non-personal |
| Event-level analytics | 13 months, then aggregate and drop | product analysis |
| Identity-linked event rows | 90 days, then pseudonymised | minimisation |
| Raw AI inference logs (Possibility) | 30 days — keep provenance, not content | cost + privacy |

**Analytics retention and product retention are different lifecycles.** Nothing in this table may be used
to justify deleting a person's Circle data (D-10).

## 8. What is not measured yet, on purpose

- No LTV/ARPU model — the inputs do not exist (`04-ECONOMICS.md` §5). Reporting one now would be fiction.
- No market share, no TAM, no penetration. **No number in this corpus may cite a market size without a source.**
- No engagement targets. If the North Star is healthy and the anti-metrics are banned, engagement follows
  or it was never real.

## 9. Open measurement questions

1. Is 3 Story entries the right MCT bar? Calibrate on the first real cohort (D-16 item 7).
2. Should `Next` completion or `Commitment` completion be the loop-closing signal? Both count today;
   choose one once data exists.
3. At what Paid Circle share does the mostly-free promise start feeling broken to users? State a threshold
   rather than trusting a feeling.
4. Do we measure Circles or people as the North Star unit? Currently Circles normalised by people —
   revisit if Circle creation becomes gameable.

