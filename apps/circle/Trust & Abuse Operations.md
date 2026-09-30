# Trust & Abuse Operations

**Status:** AGREED (policy model), BLOCKED-BY-CODE (no implementation exists)
**Why this document was missing:** the Circle set asserted "trust" in five documents
and required anti-abuse limits (`Pricing` 14), a restricted account state
(`Lifecycle` 34) and child safety (`Legal` 35) - but specified **no abuse model, no
rate limits, no Sybil defence and no moderation workflow**. Trust claims without an
operations model are marketing.

**Related:** ADR-0001 (state machines), `Legal Trust & Safety.md`, `Lifecycle &
Account Status.md` 30-34, `Identity & Privacy.md` 19-25.

---

## 1. The primary attack surface is invitations

Growth for a tight-circle product is invite-driven (`PRD` 92, 1740-1753): join, add
people, form Story. That makes **membership invitations the highest-value exploit**,
and the documents never mention it.

Threats, in order of expected impact:

| # | Threat | Why Circle is exposed | Control |
|---|---|---|---|
| T1 | Invite spam / flooding | every member of a Circle can invite | invite rate limit per Face per Circle per day; daily cap per Face; invite requires an existing relationship signal |
| T2 | Sybil Circle creation | Circle creation is free and instant | creation velocity limits; capability gating for new Circles; reputation from `trust-reputation-service` |
| T3 | Relationship-graph harvesting | a Circle is a structured list of real people and their roles | export requires role authority + rate limit + watermark/audit; **no bulk member export below ORGANIZATION** |
| T4 | Financial fraud in Vault | contribution/obligation records (Levels 1-2) can be forged or used for laundering | evidence required for initiation; transaction thresholds; anomaly detection; never Level 3 custody |
| T5 | Impersonation via Face | pseudonymous Faces are legitimate by design (`Identity & Privacy.md` 6) | verification badge is scoped per Circle context, never global (`Archetypes.md` 34) |
| T6 | Illicit or harmful content in Story | durable media, family context, minors present | moderation queue + hash matching + age-gated Circles; `Legal` 243 |
| T7 | Dormancy exploitation | ADR-0005 means data survives indefinitely | cold-tier migration + abuse of "never delete" for storage squatting: capacity policy in `Monetization & Economics` 4 |
| T8 | Possibility-engine abuse | AI surfaces people and opportunities | surfacing is permission-scoped only; no cross-Circle inference without explicit disclosure (`Interconnection & Context Engine`) |

## 2. Rate limits (starting policy)

```text
invites       : 20/day per Face, 5/day per Face per Circle, exponential backoff on rejection
circle create : 3/day per Person, 10/lifetime until reputation established
story media   : 100 uploads/day/person, 5GB/day
search        : 60 queries/min per Face (context-scoped)
export        : 1 full export/day per Circle, role-gated
api           : per-plan rate limits declared in the Capability Contract
ai inference  : CCU-metered; hard ceiling per plan (AI cost is the margin risk in
                Monetization 19)
```

All limits are configurable, enforced at the platform layer (`pkg/limiter` exists in
`combi/pkg` and is ported - so this is wiring, not construction).

## 3. Enforcement ladder

Consistent with `Lifecycle` 30-34 and ADR-0001 - enforcement **restricts**, it does not
delete.

```text
SIGNAL (automated or reported)
  -> LOG  (audit recorder already ported: combi/pkg audit + audit-log-service)
  -> LIMIT   (rate/feature reduction; user not necessarily told why, to avoid
              teaching evasion)
  -> WARN    (policy notice to the actor and, where relevant, the Circle owner)
  -> RESTRICT (account or Circle restricted; data intact; appealable)
  -> SUSPEND  (membership suspended, `Lifecycle` 34; Circle continues without them)
  -> REMOVE / CLOSE (authority required; owner transfer first if they owned a Circle)
  -> DELETE  (only by user request or legal obligation - ADR-0005)
```

Rules:

* enforcement acts on a **Face/Person in context**, not on the whole Person for one
  Circle's problem (contextual authority, `Architecture` 23);
* a moderator may not read private Circle content without a logged, scoped reason -
  `Lifecycle` 40 and `Identity & Privacy.md` 11 (context isolation) apply to staff
  too;
* every action writes to the audit log; content moderation and account enforcement
  are different audit streams;
* appeal path and response SLA must exist before launch, or RESTRICT becomes an
  un-appealable account kill, which contradicts `Pricing` 20 (no dark patterns) in
  spirit as well as law in several markets.

## 4. Minors

`Legal` 35 requires age and child-safety compliance but the domain model has no age
field, no guardian concept and no consent flow.

Required before any Family launch (Family is a **recommended launch archetype**, so
this is not theoretical):

1. Person gains `age_band` (adult / adolescent / child) with minimisation - band, not
   birth date, unless a feature needs the exact date;
2. Child accounts require verified guardian consent (reuse the ported
   `consent-privacy-service`); note `consent-privacy` has 6 impl files and is **not
   mounted** in `platform-server` (see IMPLEMENTATION_READINESS);
3. Child Faces: no public visibility, no cross-Circle discovery, no Possibility
   surfacing to strangers, no financial participation in Vault;
4. Family Circles containing minors default to closed discovery, invite-only;
5. Retention and deletion for child data stricter than section 6.

## 5. Trust signals - what we refuse

`Archetypes.md` 34 and `Identity & Privacy.md` 19-25 already say:

* verification is **scoped per context**, never a global trust score;
* archetype is not a trust score;
* a verified Business differs from an informal Fellowship, and that difference must
  be shown without ranking people.

Add as policy: **no numerical social credit visible to other users.** Trust signals
inform the engine's decisions, not a leaderboard of personhood. (CONUS has a
reputation economy for *participation*; that is a different object and must not leak
into Circle identity.)

## 6. Data retention vs safety

ADR-0005 forbids deletion for non-payment. It does not forbid deletion for cause.

| Situation | Action | Basis |
|---|---|---|
| Illegal content | remove content, preserve evidence for lawful process | legal obligation |
| Policy-violating content | remove, notify Circle owner if material | safety |
| Person deleted by request | anonymise references, preserve shared history without personal fields | `Lifecycle` 36, 40 |
| Court/regulator hold | freeze deletion, extend retention | legal obligation |
| Underage account discovered | close with guardian notification, delete child data by default | child safety |

## 7. Operational cost is a pricing input

Moderation, verification and support are named costs in `CONUS Business & Economic
Model` 59 (Operations: moderation, support, verification, administration) but Circle's
`Monetization & Economics` 4 had no line for them. Added there as **C7**; if safety
headcount is not funded by the paid segment, the mostly-free model is not honest, it
is just deferred.

## 8. Build order

1. Wire `pkg/limiter` + `pkg/security` into the invite and Circle-creation paths
   (both already ported into `combi/pkg`).
2. Mount `consent-privacy-service` and `identity-service` in `platform-server`
   (identity is already mounted; consent is not).
3. Emit `circle.member.invited` / `.rejected` events with the `Measurement.md` 5
   property set so abuse is measurable from day one.
4. Stand up the moderation queue using the ported `content` + `audit-log` services.
5. Write the appeal workflow before RESTRICT ships.
