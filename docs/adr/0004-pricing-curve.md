# ADR-0004: Pricing Curve — Flat Price with Metered Add-ons and a Rate Cap

**Status:** Proposed — **money gate, requires founder sign-off**
**Date:** 2026-09-30
**Deciders:** Founder (pending)
**Supersedes:** the doubling / `2^N` reading of the capacity ladder

## Context

Two progressive-pricing mechanisms were live in the docs at once:

- `Monetization & Capacity Strategy.md:223-234` (§8): FIRST 1M → BASE, SECOND 1M
  → **2× BASE**, THIRD 1M → **3× BASE** — i.e. `N × P` (**linear**).
- `Pricing & Tier Specification.md:263-270` (§12): Band1→P, Band2→2P, Band3→3P,
  BandN→CAP — also **linear**.
- The stated founder intent was a fee that **doubles for every million users**
  until a cap — `2^(N-1) × P` (**exponential**). At a $5 base this gives $80/mo
  at band 5, $640/mo at band 8, and **$2,560/mo at 10M users**, where the
  documents would charge $50.

The exponential variant conflicts with three written promises: `Pricing` §13
("your price will never exceed this cap"), `Pricing` §36 ("I understand what
Circle costs"), and `Monetization` §2 / `PRD` §70 (mostly-free, no paywall on
ordinary participation). A curve that doubles and a guaranteed cap cannot both
be the active mechanism — either the cap binds immediately (making the curve
decorative) or it does not (making the price absurd).

## Decision (recommended)

**Delete doubling. Adopt: flat published plan price + metered add-ons + a rate
cap on escalation.**

1. **Flat base per plan** (`FREE` / `PLUS` / `ADVANCED` / `ORGANIZATION`, per
   `Pricing` §3) — published, predictable, currency-specific per `Pricing` §24.
2. **Metered add-ons** for dimensions that create real cost (CCU per ADR-0003):
   storage beyond allowance, AI/compute beyond allowance, API calls, extra seats.
   `Pricing` §15 explicitly permits capability add-ons and `§16` permits
   bundling them.
3. **Band escalation, when used, is linear and capped** — never `2^N`. Retain
   `Monetization` §8's existing `N × P` intent only for ORGANIZATION tiers where
   infrastructure genuinely scales with membership.
4. **Rate cap: a customer's price may escalate by at most one band per billing
   year**, with notice periods and grandfathering (`Pricing` §29, §33). This
   replaces "there is a maximum price" theatre with a promise a customer can
   actually verify: *my bill cannot jump more than one step per year*.
5. **Absolute cap retained** (`Pricing` §13) for ORGANIZATION only, above which
   the relationship moves to custom/enterprise terms.

## Consequences

- Positive: satisfies `Pricing` §36's "explainable in a few sentences" test and
  preserves the §2 economic philosophy.
- Positive: predictable revenue beats an escalating curve that churns the
  customers it is trying to monetise.
- Positive: keeps `Pricing` §33 pricing-versioning simple — most customers never
  see a version change because of the rate cap.
- Negative: abandons a headline "price rises with our success" story that has
  marketing appeal but no unit-economics basis.
- Negative: requires a metering pipeline before add-ons can be invoiced at all.
- Open: actual numeric values are still undetermined — `Monetization` §10 defers
  the cap to "infrastructure economics, market testing, operating costs, payment
  processing, support requirements", none of which exist. `Pricing` §274 says the
  same. These belong in `Monetization & Economics.md` (authored in this sync) and
  must be filled from a real cost model, anchored to the only tested price points
  in the repo: CONUS Daily 100 FCFA / Pro 1,000 FCFA per month.

## Alternatives Considered

- **Doubling per million (as proposed by the founder)** — rejected for the four
  reasons in Context, plus: on a global user count it charges a 3-person Family
  Circle for other people's growth.
- **Linear `N × P` everywhere** — partially retained (ORGANIZATION only);
  rejected as the general model because a linear function of *platform* size
  still penalises small Circles for large-platform success.
- **Pure per-unit usage pricing** — rejected: unpredictable, fails §36.
- **No escalation at all (flat forever)** — rejected: ignores genuine
  infrastructure cost of large Circles, which `Monetization` §15 says must be
  funded by the organisations that consume it.
