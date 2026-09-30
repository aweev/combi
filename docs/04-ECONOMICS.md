# Economics

**Status:** STRUCTURE SETTLED · **BLOCKED-BY-CODE and PROPOSED (D-11, D-12) for every number.**
**Rule:** no price may be published until §5 is filled from measurement. A false number in a pricing
document becomes a promise to a customer.

---

## 1. Economic thesis

> **Circle is mostly free to belong to. Revenue comes from scale, capability, capacity and
> organisational need — never from ordinary human participation.**

Three constraints, each binding:
1. **No advertising** as an economic foundation in Circle's economy.
2. **No sale of personal data.**
3. **Price must stay predictable** (D-12: flat base + metered add-ons + rate cap).

**Consequence, stated honestly:** margin must come from **payer mix**, not from monetising attention.
That is a harder business, and it is the chosen one. Any document claiming this framing is "the
profitable one" is asserting something §5 has not yet measured.

## 2. Revenue architecture

| # | Stream | Payer | Status |
|---|---|---|---|
| S1 | Plan subscriptions (PLUS / ADVANCED) | individuals, families | optional, **low ARPU** |
| S2 | **Organization subscriptions** | Business, large Community, Institution | **primary** |
| S3 | Metered CCU overage | ORGANIZATION payers | scales with S2 |
| S4 | Capability add-ons | Business / Community | advanced Vault, Inventory, Finance, API, high-compute AI |
| S5 | Sponsorship / subsidy | institutions funding others' access | deferred |
| S6 | Transaction fees | only if Vault reaches Level 2 (initiate) | deferred |
| S7 | Marketplace / IP | — | explicitly later |

**S2 is the business.** S1 exists so a person can upgrade, not so the company can survive on it.

## 3. Vault levels — the risk boundary

| Level | Meaning | Regulatory posture | Scope |
|---|---|---|---|
| **1 — Record** | Circle writes "Amina paid Daniel 50,000" | **No licence required** | **V1** |
| 2 — Initiate | Circle instructs a licensed provider to move money | provider holds the obligation | later, per jurisdiction |
| 3 — Custody | Circle actually holds funds | licensing, capital, audit | **permanently out of scope** |

Excluding Level 3 removes float income, interchange and deposit economics from every forecast. That is a
**deliberate profit sacrifice** that removes licensing cost and regulatory risk, and it is why the wedge
can ship without a fintech licence conversation.

## 4. Payer-mix requirement

The mostly-free model holds **only if** the paid segment carries the free one:

```text
paid_circle_revenue  ≥  (free_member_cost × free_members) + operating_cost
```

**Not one term in that inequality currently has a value.** Until §5 is measured, this is a structure with
an opinion about solvency, not a solvency plan. This was the single largest unsubstantiated claim in the
previous corpus and it must stay labelled as such.

## 5. Cost model — the eight inputs that gate every price

```text
C1 storage cost per GB-month, hot and cold tier
C2 media processing cost per asset (image, video)
C3 AI inference cost per Possibility surfaced and per action taken
C4 compute cost per active member per month (API + realtime + sync)
C5 payment processing: % + fixed, per provider, per currency
   (mobile money and cards have materially different unit economics)
C6 support cost per paid account per month, by plan
C7 moderation / safety cost per 1,000 members  ← if safety headcount is not funded by the paid
   segment, the mostly-free model is not honest, it is just deferred
C8 egress and notification delivery

Revenue − (C1..C4, C8) − C5 − C6 − C7 = Contribution Margin per Circle
```

Measured **per Circle**, by archetype and band. Also the source of the cap in D-12 and the tiering
thresholds in `02-DOMAIN.md` §12.

**How to fill it:** ship the pilot, turn on CCU metering v0, read the numbers off real Circles. Two inputs
can be narrowed now without a pilot: hosting quotes for C1/C4, and provider rate cards for C5
(D-16 items 3–5).

## 6. Pricing by archetype (structure only — no numbers)

| Archetype | Price | Metered | Rationale |
|---|---|---|---|
| Personal context | **0, permanently** | no | the person is not the customer |
| FAMILY | **0** core; optional PLUS for storage/continuity/succession | soft allowance | never penalise a family for being large |
| FELLOWSHIP | **0** core; optional PLUS | no | a trip or a savings circle should not require a corporate subscription |
| BUSINESS | PLUS → ORGANIZATION | **yes, CCU** | the strongest paid candidate |
| COMMUNITY | 0 at small scale; ORGANIZATION when governance/treasury/scale need real infrastructure | yes, banded | |

The free/paid split follows **who consumes infrastructure**, which converts "a 200-member family is not a
200-person commercial organisation" from a sentiment into an enforceable policy.

## 7. The Founding programme — recommendation against the original design

**Not recommended: a $2,000 consumer lifetime-access fee.** Four reasons:

1. **Purchasing-power mismatch.** The only real market anchor in this company is CONUS at 100 FCFA daily /
   1,000 FCFA monthly (≈ $1.65). A $2,000 entry is roughly **1,200 months (~100 years)** of that Pro price.
   That is not early-adopter pricing; it addresses a different economic class than the stated market.
2. **Perpetual liability funded at the worst unit economics.** LTA cash is recognised once while the cost
   never ends — and the product promises permanent continuity, while the two mechanisms that normally
   absorb that cost (ads, data) are forbidden.
3. **Permanent pricing fork.** Lifetime founders sit outside every future pricing version.
4. **Bad optics.** "Buy the founding cohort of a scarce-namespace platform" invites investment-like
   expectations the product cannot honour — and D-11 already removed namespace scarcity from billing.

**Recommended instrument — Founding Circle charter (B2B, capped):**

| Parameter | Proposal |
|---|---|
| Units | 20–50, hard cap |
| Buyer | Organization / Business — **never** individuals |
| Price | 2–3× list, one-time for a **defined term** (e.g. 25 years, or life of the organisation) |
| Scope | N seats, a capacity band, named capability set, priority migration, named support |
| Recognition | permanent founding attribution on the Circle's Face |
| Liability | term-limited, so cost exposure is bounded and revenue recognition is clean |

**For individuals: status, not contract.** Permanent badge, discovery priority, and a **time-boxed** Plus
gift (e.g. 24 months). Bounded cost, reusing the recognition machinery CONUS already specifies.

## 8. CCU is a control plane, not just an invoice

Every previous mention of CCU treated it as a billing chore ("hardest gap", "blocks pricing entirely").
It is one system doing four jobs, which is why it is the highest-leverage build item:

```text
CCU metering (per Circle, per dimension)
  ├─ 1. the invoice source                 (D-11, D-12)
  ├─ 2. quota + ABAC enforcement point     (entitlements, capability limits)
  ├─ 3. abuse rate budget                  (invite limits, 05-TRUST §2)
  └─ 4. tenant isolation / noisy-neighbour defence (external tenants)
```

**Whoever measures and admits consumption decides who gets to build on top.** That is the L3 ecosystem
precondition — and it is the same code as billing. Build it once, sell it four times.

## 9. Blocking checklist before any price is published

1. Fill C1–C8 from a real pilot or a hosting estimate.
2. Sign D-11, choose CCU weights (D-16 item 2), define "active seat" (D-16 item 1).
3. Decide currency and regional tiers; tax in the billing engine, not in plan definitions.
4. Confirm payment providers and fee structure (C5) — mobile-money vs card materially changes margin.
5. Derive the ORGANIZATION cap and band table from §5 output.
6. **Build CCU metering.** Without it no price can be invoiced at all.

Until then: structure agreed, **every numeric field stays a labelled blank.**

## 10. Economic integrity rules

- **Sponsorship must never become editorial control** in products that carry it (applies to CONUS; see `07-CONUS.md` §5).
- **No dark patterns.** Cancellation is as easy as creation; no retention mazes; no hidden capacity limits.
- **Capacity language is plain.** Never show a customer "84% of relational-object quota." Show
  "You've used most of your included Circle capacity," then name what is approaching its limit (D-16 item 9).
- **A cap is not unlimited abuse.** Anti-abuse, security and reasonable-use limits still apply inside a capped price.
- **Cross-subsidy is a decision, not an accident.** Business/Organization revenue funding free
  participation is the model; it must never become a paywall placed on ordinary human belonging.

