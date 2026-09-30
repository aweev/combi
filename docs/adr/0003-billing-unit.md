# ADR-0003: Billing Unit

**Status:** Proposed — **money gate, requires founder sign-off**
**Date:** 2026-09-30
**Deciders:** Founder (pending)

## Context

The commercial unit of Circle is defined four mutually exclusive ways:

| Source | Unit |
|---|---|
| `Monetization & Capacity Strategy.md:46-55` (§3) | "a Circle's active participants / capacity consumption" — explicitly *not* one subscription per person |
| `PRD.md:1425-1437` (§73) | Open question: person / account / Circle / seat / capacity allocation, with a warning that "the billing architecture must never ambiguously mix these concepts" |
| `Pricing & Tier Specification.md:248-256` (§11) | "The actual unit must be finalized" — offers people, active subscribers, or capacity units |
| `Monetization & Capacity Strategy.md:489-503` (§24) | A **scarce account-number / identifier** namespace as the thing being rationed |

A per-seat model, a per-Circle capacity model and a scarce-identifier model
produce *opposite* behaviour for the same customer. Until this is fixed, price,
capacity bands (`Pricing` §11), the cap (`Pricing` §13) and the whole inactivity
lifecycle (`Lifecycle` §12-16) cannot be computed or implemented.

## Decision (recommended)

**The billable unit is the Circle's capacity consumption, measured as
`Circle Capacity Units` (CCU).**

- **CCU metered dimensions:** active seats, storage, media/ingest volume,
  AI/compute usage, transactions processed, API calls, integrations connected.
  These map to real cost lines (`Pricing` §9 lists the dimensions;
  `Monetization` §6 lists Capacity/Power/Infrastructure as what to charge for).
- **Who is billed:** the `BILLING ACCOUNT` that owns a Circle — never a person
  for merely belonging to one. `PRD` §79's separation of
  PERSON / CIRCLE / BILLING ACCOUNT / PLAN / SUBSCRIPTION / ENTITLEMENT / USAGE
  / CAPACITY is adopted verbatim as the entity model.
- **Never billable:** the identifier namespace. Scarcity of account numbers is a
  *technical* capacity-planning concern and must not leak into pricing. This
  deliberately separates `Monetization` §24 from the billing model.
- **Price floor:** `Personal` (per ADR-0002), `FAMILY` and `FELLOWSHIP` sit at
  **zero price** with a generous fixed allowance. `BUSINESS` and large
  `COMMUNITY` are the metered segments. This honours `Monetization` §20:
  "a Circle with 200 family members should not automatically be treated like a
  200-person commercial organization."

## Consequences

- Positive: one unit, one formula; `getPrice(archetype, plan, capacity,
  capabilities, region, billingPeriod)` (`Pricing` §32) becomes implementable.
- Positive: a Family cannot be charged into abandoning the product by growing;
  retention risk from headcount is removed.
- Positive: decoupling the identifier namespace from billing removes the
  incentive to delete dormant accounts for revenue reasons (see ADR-0005).
- Negative: CCU requires real metering infrastructure (usage aggregation per
  Circle). None exists yet: `payments-service` + `billing-subscriptions-service`
  total 11 Go files and neither is mounted in `platform-server`.
- Negative: usage-based pricing is harder to forecast for customers than flat
  pricing; mitigated by ADR-0004's flat base + rate cap.
- Risk: "active seat" needs a definition (what makes a seat active in a billing
  period?) before invoicing can be automated.

## Alternatives Considered

- **Per-person subscription** — rejected: contradicts `Monetization` §3 and
  would charge a person for belonging to multiple Circles.
- **Per-Circle flat price by size band only** — viable and simpler, but ignores
  storage/AI cost drivers, so margin collapses on media-heavy or AI-heavy
  Circles; retained as the *base* of the recommended hybrid.
- **Scarce-identifier rationing as the unit** — rejected: monetises a technical
  limitation, and the only document proposing it (`Monetization` §24-26) is also
  the one containing the orphaned waitlist mechanism that this sync deletes.
- **Pure usage-based (no flat base)** — rejected: unpredictable invoices violate
  `Pricing` §36 ("I understand what Circle costs").
