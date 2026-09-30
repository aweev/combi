# ADR-0002: Circle Archetype Set

**Status:** Accepted
**Date:** 2026-09-30
**Deciders:** Docs sync (wording conflict arbitration)
**Supersedes:** `PRD.md:469-497` (§20 "Individual / Me"), `Archetypes & Circle Types.md:41-50` ("PERSON")

## Context

Four documents listed different archetype sets:

| Source | Set |
|---|---|
| `PRD.md` §20 | **Individual / Me**, Family, Business, Fellowship, Community |
| `Archetypes & Circle Types.md` §3 | **PERSON**, Family, Fellowship, Business, Community |
| `Domain Model.md` §7 | Family, Fellowship, Business, Community (no personal) |
| `Navigation & Experience Architecture.md` §3 | `ME` shown *above* `CIRCLES`, which contains Family, Business, Fellowship, Community |
| `Monetization & Capacity Strategy.md` §4 | **Personal**, Family, Fellowship, Business, Community |

Five names (`Individual`, `Me`, `Person`, `Personal`, `Face`) were used
interchangeably for one concept. The consequence with money attached:
`Pricing & Tier Specification.md` defines §27 Business, §28 Family, §29
Fellowship and §30 Community pricing — and **has no section for the personal
context at all**, which is the largest cohort in the product.

## Decision

**A Circle has exactly four archetypes: `FAMILY`, `FELLOWSHIP`, `BUSINESS`,
`COMMUNITY`.**

The person is **not** a fifth archetype. `Domain Model.md` §7 is canonical
because it is the only version consistent with the rest of the model: a Circle
is defined as a *collective* context (`Domain Model` §6, `Core Objects` §4), so
a single person cannot be one. The individual context is served by the existing
`Person` + `Personal Face` primitives plus `Navigation`'s `ME` surface
(Story/Now/Next/Possibilities), which already covers everything "Individual /
Me" was meant to express.

`Individual` and `Me` are retired as *model* terms; `Me` remains acceptable as
a navigation label only (it already is one in `Navigation` §3). `Personal`
survives only as an adjective for the Face type (`ADR-0007`).

## Consequences

- Positive: archetype count matches the tree in `Navigation` §3 and the entity
  table in `Domain Model` §7, so the taxonomy is consistent across the set.
- Positive: makes the archetype/plan decoupling in `Pricing` §8 ("the plan is
  the commercial layer, the archetype determines the experience") cleanly
  expressible — 4 archetypes × 4 plans, not a confused 5×5.
- Negative: the personal context now needs its own pricing prose; added as
  `Pricing & Tier Specification.md` §27a in this sync.
- Negative: `Monetization` §4's "Personal" heading must be relabelled; done.
- Risk: an "individual-only" Circle (a private journal Circle) may be requested
  later. It should be modelled as a Circle with one member and an explicit
  archetype decision, not by reviving a fifth archetype.

## Alternatives Considered

- **Five archetypes with `Individual`** — rejected: contradicts "a Circle is a
  collective context" and forces a fifth pricing chapter for a concept that
  `Person`/`Face` already own.
- **Keep `PERSON` as an archetype** (`Archetypes & Circle Types` §3) — rejected:
  `PERSON` is a primitive in `Core Objects` §2, and promoting a primitive to an
  archetype creates two ways to represent the same thing.
- **Three archetypes (drop Fellowship)** — rejected: Fellowship has its own
  roles (§ organizer), its own pricing section and is the recommended launch
  wedge; nothing supports removing it.
