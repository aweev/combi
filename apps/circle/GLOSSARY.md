# Circle Glossary — Controlled Vocabulary

**Authority:** this file is the controlled list of names. A term not listed here
must not be introduced into user-facing copy or into a schema without a change
request (ADR-0010). Where documents used different words for one thing, this file
states the winner and lists the rejected synonyms.

---

## 1. User-facing vocabulary (exactly 7 — ADR-0010)

| Term | Definition | Rejected synonyms |
|---|---|---|
| **Circle** | A meaningful shared context between people. A collective, not a person. | group, team, space, community (as a generic) |
| **Face** | A context-specific representation of a Person — the living front door into Circle. | profile, persona, account |
| **Story** | What has happened: preserved, meaningful history for a person or Circle. | feed, timeline, posts, journal |
| **Now** | What is happening: current activity, commitments and attention. | dashboard, inbox, activity feed |
| **Next** | What is intended: plans, intentions and upcoming direction. | goals, roadmap, backlog |
| **Vault** | A Circle's financial context: record, coordinate, reconcile. | wallet, bank, treasury (as UI) |
| **Possibility** | A contextually surfaced opportunity, with provenance. | suggestion, recommendation, discovery |

Note the deliberate non-words: Circle has **no feed**, **no followers**, **no
posts** and **no likes** (`PRD` §6, `Identity & Privacy.md` §21). Calling
Story a "feed" in any document or spec is a defect, not a synonym.

## 2. Identity terms

| Term | Definition |
|---|---|
| **Person** | The underlying human being. A primitive, **not** an archetype (ADR-0002). |
| **Account** | Access to Circle: credentials, devices, security, subscription, lifecycle. Not the person's identity. |
| **Personal Face** | Face used for personal participation. |
| **Business Face** | Face used when acting on behalf of a business. |
| **Pseudonymous Face** | Face used when intentionally participating without ordinary identity. |
| **Face visibility** | Public/discoverable settings — a **level**, never a Face type (ADR-0007). |
| **Private Space** | A mode of a Personal Face: Story/Next owned by the person, belonging to no Circle. Not a primitive. |
| **Membership** | The link Person/Face → Circle, carrying role, status, join date, permissions. |
| **Role** | What someone may do inside one context. Authority never crosses Circles. |

## 3. Archetypes (exactly 4 — ADR-0002)

| Term | Definition |
|---|---|
| **Family** | A family or household context. |
| **Fellowship** | A group united around shared participation, interest, faith, activity or purpose. |
| **Business** | A commercial or professional organization. |
| **Community** | A broader collective context. |

Retired as model terms: `Individual`, `Me`, `Personal` (as an archetype), `PERSON`
(in the archetype list). "ME" survives only as a navigation label.

## 4. Structure (ADR-0008)

| Term | Definition |
|---|---|
| **Branch** | Parent → child Circle, **depth capped at 1**. The child is a Circle in its own right. |
| **Affiliation** | Peer link between two Circles; each side explicitly chooses what it discloses. |
| ~~Intentional audience layer~~ | **Removed.** No such object exists in the domain model. |
| ~~Nested Circles (unbounded)~~ | **Not supported.** Use Branch or Affiliation. |

## 5. Plan and billing terms (ADR-0003, ADR-0004)

| Term | Definition |
|---|---|
| **Plan** | The commercial layer: `FREE`, `PLUS`, `ADVANCED`, `ORGANIZATION`. |
| **Archetype ≠ Plan** | The plan is what you pay for; the archetype is how the experience is configured (`Pricing` §8). |
| **CCU** | **Circle Capacity Unit** — the billable unit. Metered: active seats, storage, media/ingest, AI/compute, transactions, API calls, integrations. |
| **Billing Account** | The entity that pays. Never a person merely for belonging to a Circle. |
| **Entitlement** | What a subscription grants. Distinct from usage and from capacity. |
| **Rate cap** | A bill may escalate by at most one band per billing year. |
| **Absolute cap** | ORGANIZATION-only maximum price; above it, custom terms. |
| **Limited Mode** | User-facing label for the `RESTRICTED` subscription state. Code says `RESTRICTED`. |

## 6. Lifecycle state names (ADR-0001)

```text
Subscription  TRIAL · ACTIVE · PAST_DUE · GRACE · RESTRICTED · INACTIVE · CLOSED
Account       INVITED · CREATED · ACTIVE · INACTIVE · DORMANT · CLOSED · DELETED
Data          ACTIVE · ARCHIVED · DELETION_PENDING · DELETED
Membership    INVITED · PENDING · ACTIVE · PAUSED · SUSPENDED · LEFT · REMOVED
Capability    AVAILABLE · ENABLED · DISABLED · RESTRICTED · DEPRECATED
Identifier    AVAILABLE · ASSIGNED · ACTIVE  (reclamation prohibited — ADR-0005)
```

Banned as subscription states (duplicates, or belonging to another machine):
`GRACE_PERIOD`, `TERMINATED`, `ARCHIVED`, `RETRY`, `CANCELLED`, `EXPIRED`,
`DELETION_PENDING`, `DELETED`.

"Inactive subscription" means: paid plan expired, renewal not completed, no
qualifying reactivation, grace exhausted. It **never** means "did not open the app"
(`Lifecycle` §12).

## 7. Financial levels (Vault — do not conflate)

| Level | Meaning | Regulatory posture |
|---|---|---|
| **Level 1 — Record** | Circle writes that "Amina paid Daniel 50,000." | No licence required. **V1 scope.** |
| **Level 2 — Initiate** | Circle instructs a licensed provider to move money. | Provider holds the obligation. |
| **Level 3 — Custody** | Circle actually holds funds. | **Out of scope.** Requires licensing, capital, audit (`Vault` §3, §66). |

## 8. Locales (ADR-0009)

```text
en-NG       Nigerian English        default
fr-CM       Cameroonian French
pcm-NG      Nigerian Pidgin         first-class, not "optional"
ha-NG-Arab  Hausa in Ajami script   RTL — mandatory layout requirement
```

## 9. Platform names (ADR-0006, proposed)

| Name | Meaning |
|---|---|
| **Combi** | The company and monorepo (`github.com/combi/*`, DB `combi`). |
| **Circle** | The flagship product. |
| **CONUS** | An application built on Circle primitives (media / participation). |
| **Cast** | An application family: arena, burnbook, cortex, courtroom, creator-studio, squads, syndicate. |
| **NGO** | Legacy framing of the `api/` codebase. History, not a product line. |
| **Africa OS** | Name of the platform-infrastructure specs in `doc/`. |

## 10. Open naming questions

Flagged rather than invented — not resolved by this sync:

1. **"Face"** — retained, but unusual as a public term; a user study would tell us
   whether it survives.
2. **"Circle" vs "Circles"** — singular as concept, plural as navigation
   destination; copy guidelines should fix the convention.
3. **CCU display name** — customers should never see "CCU". A plain-language
   capacity label is still to be designed (`Pricing` §10 requires it).
4. **Vault vs treasury** — `Monetization` §4 says "treasury" for Community while
   `Vault` owns the concept. Pick one in copy.
5. **Possibility vs Possibilities** — `PRD` §16 and Navigation use the plural for
   the surface, the singular for one item. Keep, but state it in copy guidelines.
