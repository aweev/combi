# Circle Analytics & Measurement Specification

**Status:** AGREED (V1 metrics) — fills the gap `PRD` §98 delegated to a file that
did not exist.
**Authority:** measurement only. It does not redefine product behaviour.

Companion model: this is Circle's counterpart to `apps/conus/Measurement.md`
(which defines WQP for CONUS). Structure and event discipline are borrowed
deliberately from that document, because it is the only measurement spec in this
company that has been thought through end to end.

---

## 1. Measurement principle

Per `PRD` §84 and §99, Circle measures product health **without turning human
relationships into engagement bait**:

* we do **not** optimize DAU, session length, notifications sent, or impressions;
* we **do** optimize whether something meaningful was preserved, understood or
  completed.

## 2. North Star metric

> **Continuing Circles per 1,000 people** — Circles meeting the **Meaningful
> Continuity Threshold (MCT)** at day 60, per 1,000 members.

**MCT — a Circle counts when all are true at day 60 after creation:**

1. ≥ 2 members with `ACTIVE` membership;
2. ≥ 3 Story entries created by ≥ 2 distinct members;
3. ≥ 1 `Next` item created or updated in the last 30 days;
4. ≥ 1 completed interaction attributable to the loop (commitment fulfilled,
   contribution recorded, reply to a `Now` item, or action on a Possibility).

This operationalises `PRD` §98 ("meaningful continuity created for people and
Circles") and the `Product Philosophy` §21 loop. Criterion 4 is the loop test: a
Circle that only accumulates posts but never acts has not closed the loop.

**Why "per 1,000 people" instead of an absolute count:** an absolute number grows
with sign-ups, which rewards acquisition over health. A rate forces the product to
make Circles work rather than merely create them.

## 3. Supporting metrics

| Tier | Metric | Definition | Guards against |
|---|---|---|---|
| Activation | Circle Creation Success | new people with a ≥2-member Circle within 7 days | sign-up churn |
| Activation | Second-Person Rate | new Circles reaching 2 active members in 14 days | empty-shell Circles |
| Continuity | Story Density | median Story entries per member at day 60 | thin records |
| Continuity | Recall Use | % of members opening Story (not Now) weekly | feed-shape drift |
| Loop | Commitment Completion | % of `Next` items closed rather than expired | intention debt |
| Loop | Possibility Acceptance | % of surfaced Possibilities acted on | unhelpful AI |
| Financial | Vault Activity | % of Circles with ≥1 recorded transaction in 30 days | the wedge failing |
| Retention | Circle Survival @ 180d | % of MCT Circles still MCT at day 180 | vanity launches |
| Monetization | Paid Circle Share | % of Circles on paid plans, **by archetype** | cannibalising the free core |
| Monetization | Expansion by Band | band escalations per quarter vs the rate cap | unexpected bill jumps (ADR-0004) |

## 4. Anti-metrics (explicitly not optimized)

Per `PRD` §99, these are banned as targets: raw DAU/MAU ratio, time-in-app,
notification volume or open rate as a goal, posts per user, connection/follower
counts, and streak mechanics that reward opening the app without doing anything.

**Rule:** if a §3 metric can be improved by any of the above, the metric is wrong.
Fix the metric, not the behaviour.

## 5. Event model

Naming follows the platform convention already in code
(`pkg/domain/core/events.go`: `financial.subscription.created`,
`financial.donation.received`), i.e. `<domain>.<object>.<past-tense-action>`.

Required new Circle events (**none are emitted today** — see
`IMPLEMENTATION_READINESS.md` §4):

```text
circle.created
circle.member.joined              circle.member.left
circle.story.entry_created        circle.story.entry_shared
circle.now.item_completed         circle.next.item_created
circle.next.item_completed        circle.possibility.surfaced
circle.possibility.acted          circle.vault.transaction_recorded
circle.vault.contribution_made    circle.capacity.threshold_reached
subscription.lifecycle.changed    account.lifecycle.changed
data.lifecycle.changed
```

**Required properties:** `circle_id`, `actor_face_id` (never a raw person id in
analytics), `archetype`, `plan`, `capacity_band`, `locale`, `region`, `trace_id`,
`emitted_at`. Emit through `events.Emit` so request/tenant/trace metadata come from
context (`task.md` Phase 0).

**Identity rule:** `circle.member.left` and account-lifecycle events carry no
personal data beyond pseudonymous ids, because they are the events most likely to
outlive the person who triggered them (`Lifecycle` §36, §40).

## 6. Analytics retention (draft — needs legal review)

| Data class | Retention | Basis |
|---|---|---|
| Aggregate daily rollups | Indefinite | Non-personal |
| Event-level analytics | 13 months, then aggregate and drop | Product analysis |
| Identity-linked event rows | 90 days, then pseudonymised | Minimisation |
| Raw AI inference logs (Possibility) | 30 days — keep provenance, not content | Cost + privacy |

`Legal Trust & Safety.md` covers minors, moderation and copyright but contained **no
retention schedule**, which is why this table exists here. ADR-0005 constrains it:
nothing in this table may be used to justify deleting a person's Circle data —
analytics retention and product data retention are different lifecycles.

## 7. Dashboards

1. **Continuity health** — MCT Circles per 1,000 people, weekly, split by archetype.
2. **Loop closure** — Possibility surfaced → acted → Story entry created.
3. **Lapse funnel** — `PAST_DUE → GRACE → RESTRICTED → INACTIVE → CLOSED`, which is
   the instrument that proves ADR-0005 is holding: data loss must read zero.
4. **Capacity vs cost** — CCU consumed per plan against real infrastructure cost.
   This is the input the economics model needs before any price is committed.
5. **Locale parity** — MCT rate per locale, to catch `pcm-NG` or `ha-NG-Arab`
   users receiving a weaker experience (`Language` §28 forbids it).

## 8. Open measurement questions

1. Is 3 Story entries the right MCT bar? Calibrate on the first real cohort.
2. Should `Next` completion or `Commitment` completion be the loop-closing signal?
   Both count today; choose one once data exists.
3. At what Paid Circle share does the mostly-free promise start feeling broken to
   users? State a threshold rather than trusting a feeling.
4. Do we measure Circles or people as the unit of the North Star? Currently
   Circles, normalised by people — revisit if Circle creation becomes gameable.

