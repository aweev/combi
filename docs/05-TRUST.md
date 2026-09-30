# Trust & Safety Operations

**Status:** POLICY SETTLED · BLOCKED-BY-CODE (nothing wired).
**Why it exists:** trust claims without an operations model are marketing. The previous corpus asserted
"trust" in five documents, required anti-abuse limits and child safety — and specified no abuse model, no
rate limits, no Sybil defence and no moderation workflow.
**Rule:** §2 and §4 ship **before** any growth loop is opened. Not alongside. Before.

---

## 1. The primary attack surface is the invitation

Growth for a tight-circle product is invite-driven (Manifest §5): join, add people, form Story. That makes
**membership invitations the highest-value exploit**, and no older document mentioned it.

| # | Threat | Why Circle is exposed | Control |
|---|---|---|---|
| T1 | Invite spam / flooding | every member of a Circle can invite | invite rate limit per Face per Circle per day; daily cap per Face; invite requires an existing relationship signal |
| T2 | Sybil Circle creation | Circle creation is free and instant | creation velocity limits; capability gating for new Circles |
| T3 | Relationship-graph harvesting | a Circle **is** a structured list of real people and their roles | export requires role authority + rate limit + audit; **no bulk member export below ORGANIZATION** |
| T4 | Financial fraud in Vault | contribution/obligation records can be forged or used to launder | evidence required for initiation; transaction thresholds; anomaly detection; **never Level 3 custody** |
| T5 | Impersonation via Face | pseudonymous Faces are legitimate by design (D-02) | verification badge is **scoped per Circle**, never global |
| T6 | Harmful content in Story | durable media, family context, minors present | moderation queue + hash matching + closed-by-default Circles containing minors |
| T7 | Dormancy squatting | D-10 means data survives indefinitely | cold-tier migration + capacity policy priced in `04-ECONOMICS.md` §5 |
| T8 | Possibility-engine abuse | AI surfaces people and opportunities | surfacing is permission-scoped only; **no cross-Circle inference without explicit disclosure** |

## 2. Rate limits — starting policy

```text
invites        : 20/day per Face, 5/day per Face per Circle, exponential backoff on rejection
circle create  : 3/day per Person, 10/lifetime until reputation is established
story media    : 100 uploads/day/person, 5GB/day
search         : 60 queries/min per Face, context-scoped
export         : 1 full export/day per Circle, role-gated
api            : per-plan limits declared in the Capability Manifest
ai inference   : CCU-metered, hard ceiling per plan (AI cost is the margin risk, 04-ECONOMICS C3)
```

All values configurable, enforced at the platform layer. **`pkg/limiter` is already ported — this is
wiring, not construction.** Nothing currently calls it for invitations. That gap is T1, live.

## 3. Enforcement ladder

Consistent with D-06: enforcement **restricts, it does not delete.**

```text
SIGNAL (automated or reported)
  → LOG      every action to the audit recorder
  → LIMIT    rate/feature reduction; the actor is not necessarily told why, to avoid teaching evasion
  → WARN     policy notice to the actor
  → RESTRICT capability or membership suspended; data intact, export still available
  → CLOSE    account closed; shared history anonymised, not destroyed
```

Staff rules:
- staff may **not** read private Circle content without a logged, scoped reason — context isolation
  applies to employees and to the platform's own systems, not just to users;
- every action writes to the audit log; **content moderation and account enforcement are different audit streams**;
- **an appeal path and response SLA must exist before `RESTRICT` ships.** Without them `RESTRICT` is an
  un-appealable account kill, which contradicts the no-dark-patterns rule in spirit as well as in law in
  several markets.
## 4. Minors — required before any Family-facing surface

Family is a launch archetype; minors are expected in it. The model currently has **no age field, no
guardian concept, no consent flow.** Required:

1. `Person.age_band` (adult / adolescent / child) — **band, not birth date**, unless a feature needs the
   exact date (data minimisation).
2. Child accounts require verified guardian consent, reusing the ported `consent-privacy-service`.
   **It is ported but not currently mounted in `platform-server`** (`08-STATUS.md` §4).
3. Child Faces: no public visibility, no cross-Circle discovery, no Possibility surfacing to strangers,
   **no financial participation in Vault.**
4. Family Circles containing minors default to **closed discovery, invite-only.**
5. Retention and deletion rules for child data are **stricter** than §5.

## 5. Retention vs. safety

D-10 forbids deletion for non-payment. **It does not forbid deletion for cause.**

| Situation | Action | Basis |
|---|---|---|
| Illegal content | remove content, preserve evidence for lawful process | legal obligation |
| Policy-violating content | remove; notify Circle owner if material | safety |
| Person deleted by request | **anonymise** references, preserve shared history without personal fields | continuity + minimisation |
| Court / regulator hold | freeze deletion, extend retention | legal obligation |
| Underage account discovered | close with guardian notification, **delete child data by default** | child safety |

The reconciliation of "never delete for non-payment" with "do not hoard personal data indefinitely" is:
**anonymise, don't erase.** Analytics retention is a separate lifecycle defined in `02-DOMAIN.md` §12 —
nothing there may be used to justify deleting a Circle's product data.

## 6. Trust signals — what we refuse

- Verification is **scoped per context**. Never a global trust score.
- Archetype is not a trust signal. A verified Business differs from an informal Fellowship, and that
  difference must be *shown* without ranking people.
- **No numerical social credit visible to other users.** Trust signals inform the engine's decisions, not
  a leaderboard of personhood.
- Trust comes from reality: real transactions, completed work, resolved disputes, actual participation,
  explicit vouches, evidence. **Not** likes, follower counts, popularity or arbitrary scores.
- Family and Fellowship closeness is **never gamified.**
- CONUS has a reputation economy for *participation*. That is a different object and **must not leak into
  Circle identity** (`07-CONUS.md` §6).

## 7. Deletion, export and departure

- **Leaving a Circle:** the person's contributions and history remain in the Circle's Story (it is shared
  history), attributed to the Face, with personal fields available for minimisation on request.
- **Removing a person:** requires a Role that grants it, writes an audit event, and notifies.
- **Dissolution:** a CLOSED Circle's data is preserved subject to D-10; export remains available.
- **Succession:** a Family Circle needs a documented successor path — the record outliving its founder is
  the entire value proposition.
- **Export** is machine-readable and includes capability-owned records. Portability is what makes trust
  credible rather than coercive.

## 8. Build order (a schedule, not an aspiration)

1. Wire `pkg/limiter` + `pkg/security` into the invite and Circle-creation paths — **both already ported.**
2. Mount `consent-privacy-service` in `platform-server`; add `age_band` + guardian consent.
3. Emit `circle.member.invited` / `.rejected` with the standard property set, so abuse is measurable day one.
4. Stand up the moderation queue using the ported `content` + `audit-log` services.
5. Write the appeal workflow **before** `RESTRICT` ships.
6. Operational cost of all of the above enters the cost model as **C7** (`04-ECONOMICS.md` §5).

## 9. Legal posture (short, because the long version was 620 lines of caution)

- **Regulatory data is real and already in code:** `pkg/compliance` ships per-country KYC tiers with named
  citations (e.g. COBAC R-2019/01 Art. 14/15 for Cameroon) and transaction limits across NG, CM, KE, GH,
  ET, SN, TZ, UG, plus sanctions sources (UN SC, OFAC SDN, EU FSF, COBAC AML, CBN BVL, CBK CRB).
  **Read it before designing any Vault-adjacent flow** — it is the most expensive thing in the repo to
  rebuild and currently has no consumer.
- **Level 1 record-only keeps us outside most of it.** Do not drift toward Level 2 without a per-jurisdiction
  legal review against those tiers.
- Minors: age-band consent per §4, and the stricter jurisdiction's rule wins by default.
- Moderation, takedown, reporting and appeals: §3 is the workflow; **report ≠ takedown** — a report
  triggers review, not deletion.
- Copyright: content ownership stays with the creator/Face; the platform holds a licence to store and
  render, nothing more.
- This document is not legal advice and **has not been reviewed by counsel.** The §8 build order is the
  engineering consequence of the policy, not a compliance certification.

