# Circle Monetization & Economics Specification

**Status:** BLOCKED-BY-CODE for numbers; AGREED for structure.
**Resolves:** the dangling references formerly at `PRD` §72, `Pricing` §12 and
`Data Architecture` §46, each of which pointed at a specification that did not exist.
**Depends on:** ADR-0003 (billing unit), ADR-0004 (pricing curve), ADR-0006 (brand).

---

## 1. What this document is for

Every pricing document here defers numbers to "financial modeling". That model does
not exist yet, so this file defines **the method and the structure** and states
plainly which inputs are missing. Publishing prices before §4 is filled would be
guesswork dressed as a decision.

## 2. Economic thesis (unchanged from `Monetization` §1–§2)

> Circle is mostly free to belong to. Revenue comes from scale, capability, capacity
> and organizational need — never from ordinary human participation.

Three constraints shape every number below:

* no advertising as an economic foundation (`Monetization` §18);
* no sale of personal data (`Monetization` §17);
* price must stay predictable (`Pricing` §13, §36) with escalation capped (ADR-0004).

Consequence: **margin must come from payer mix, not from monetising attention.** That
is a harder business, and it is the chosen one.

## 3. Revenue architecture

| Stream | Payer | Status |
|---|---|---|
| S1 Plan subscriptions | PLUS / ADVANCED individuals and families | optional, low ARPU |
| S2 **Organization subscriptions** | Business, large Community, Institution | **primary** |
| S3 Metered CCU overage | ORGANIZATION payers | scales with S2 |
| S4 Capability add-ons | Business / Community | advanced Cargo, Inventory, Finance, API, high-compute AI (`Pricing` §15) |
| S5 Sponsorship / subsidy | institutions funding others' access | `Monetization` §16 — deferred |
| S6 Transaction fees | only if Vault reaches Level 2 (initiate) | deferred |
| S7 Marketplace / IP | — | explicitly later |

`Vault` **Level 3 (custody) is excluded**, so no float income, no interchange and no
deposit economics appear in any forecast. This is a deliberate profit sacrifice that
removes licensing cost and regulatory risk (`Vault` §66).

## 4. Cost model — the method (inputs required)

Per-CCU cost must be measured, not assumed. Required inputs, none of which exist yet:

```text
C1 storage cost per GB-month, hot and cold tier
C2 media processing cost per asset (image, video)
C3 AI inference cost per Possibility surfaced and per action taken
C4 compute cost per active member per month (API + realtime + sync)
C5 payment processing: % + fixed, per provider, per currency
   (mobile money and cards have materially different unit economics)
C6 support cost per paid account per month, by plan
C7 moderation / safety cost per 1,000 members (Trust & Abuse Operations.md)
C8 egress and notification delivery
```

**Unit economics formula**, adapted from `CONUS Business & Economic Model.md` §45
which already states this discipline:

```text
Revenue
  − infra (C1..C4, C8)
  − payment costs (C5)
  − support (C6)
  − safety/moderation (C7)
  = Contribution Margin per Circle
```

Measured per Circle, by archetype and band. `Monetization` §10 requires precisely
these inputs to set the cap, so the cap cannot be set until this table is filled.

## 5. Payer-mix requirement

The mostly-free model works only if the paid segment carries the free one:

```text
paid_circle_revenue ≥ (free_member_cost × free_members) + operating cost
```

| Assumption | Value | How to establish |
|---|---|---|
| Free member cost | **unknown** | needs §4 C1–C4; instrument a pilot cohort |
| Business paid conversion | **unknown** | pilot |
| ORGANIZATION ARPU | must exceed the free-member cost it carries, with margin | pilot |
| Churn | rate cap (ADR-0004) removes bill-shock churn; retention becomes product-driven | pilot |


The only real market anchor in this company is CONUS: **100 FCFA daily / 1,000 FCFA
monthly (~$1.65)**. A $2,000 consumer entry is roughly **1,200 months (about 100
years)** of that Pro price — so it is not early-adopter pricing; it addresses a
different economic class than the stated market.

## 6. Pricing by archetype (structure, per ADR-0002 / ADR-0003)

| Archetype | Price | Metered? | Rationale |
|---|---|---|---|
| Personal context | **0, permanently** | no | `Monetization` §4, §5 |
| FAMILY | **0** core; optional PLUS for storage, continuity, succession | soft allowance | `Pricing` §28 — never penalise a family for being large |
| FELLOWSHIP | **0** | no | `Pricing` §29 — a trip should not require a corporate subscription |
| BUSINESS | PLUS → ORGANIZATION, CCU-metered | **yes** | `Monetization` §4 names it the strongest paid candidate |
| COMMUNITY | 0 at small scale; ORGANIZATION when governance/treasury/scale need real infrastructure | yes, banded | `Pricing` §30 |

The free/paid split follows **who consumes infrastructure**, which is how
`Monetization` §20's fairness rule ("200 family members ≠ a 200-person commercial
organization") becomes an enforceable policy instead of a sentiment.

## 7. Founder / early-access programme (revised recommendation)

A $2,000 **consumer** lifetime access fee is not recommended:

1. **Purchasing-power mismatch** — see §5: ~100 years of the company's tested Pro price.
2. **Perpetual liability funded at the worst unit economics.** LTA cash is recognised
   once while the cost never ends — the product promises permanent continuity (`Vault`,
   Story) and the two normal absorbers of that cost are forbidden (no ads §18, no data
   sales §17).
3. **Permanent pricing fork.** Founders sit outside every future pricing version
   (`Pricing` §33) where §35 requires a billing *rule*, not a product fork.
4. **Bad optics.** "Buy the founding cohort of a scarce-namespace platform" invites
   investment-like expectations the product cannot honour.

**Recommended instrument — Founding Circle charter (B2B, capped):**

| Parameter | Proposal |
|---|---|
| Units | 20–50, hard cap |
| Buyer | Organization / Business — **never** individuals |
| Price | 2–3× list, one-time for a defined term (e.g. 25 years, or life of the organization) |
| Scope | N seats, a capacity band, named capability set (advanced Vault, governance, reporting), priority migration, named support |
| Recognition | permanent founding attribution on the Circle's Face |
| Liability | term-limited, so cost exposure is bounded and revenue recognition is clean |

**For individuals: status, not contract.** Permanent badge, discovery priority, and a
**time-boxed** Plus gift (e.g. 24 months). Bounded cost, reusing the recognition and
reward machinery this company already specifies (`CONUS` §29–§34).

## 8. Blocking checklist before any price is published

1. Fill C1–C8 in §4 from a real pilot or a hosting estimate.
2. Choose CCU weights (ADR-0003 lists dimensions but assigns no priority).
3. Decide currency and regional tiers (`Pricing` §24–§26), with tax handled in the
   billing engine rather than in plan definitions.
4. Confirm payment providers and fee structure (C5) — mobile-money fees are not card
   fees, and that materially changes the margin.
5. Derive the ORGANIZATION absolute cap and band table from §4 output.

Until then, **the structure here is agreed and every numeric field stays a labelled
blank.** A false number in a pricing document becomes a promise to a customer.
