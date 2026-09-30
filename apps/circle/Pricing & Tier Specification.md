# CIRCLE

## Pricing & Tier Specification

### Translating the economic model into an understandable product structure

---

# 1. Purpose

This document defines how Circle's monetization model is presented to users.

The pricing system must remain:

* simple;
* predictable;
* scalable;
* archetype-aware;
* capacity-aware.

The user should not need to understand Circle's internal economics.

---

# 2. Pricing Principles

### Principle 1

Free should be genuinely useful.

### Principle 2

Paid plans should represent meaningful additional value.

### Principle 3

Pricing should reflect context.

### Principle 4

Capacity should be understandable.

### Principle 5

Users should never feel trapped.

### Principle 6

Inactive accounts should have a predictable lifecycle.

---

# 3. Plan Architecture

Rather than creating dozens of plans, Circle should begin with a small number.

Conceptually:

```text
FREE
↓
PLUS
↓
ADVANCED
↓
ORGANIZATION
```

These are implementation placeholders, not final names.

---

# 4. Free

Purpose:

> Belong and build your Circle.

Core access:

* Personal Face;
* basic Story;
* Now;
* Next;
* Possibilities;
* basic Circles;
* basic relationships;
* basic communication;
* basic media;
* basic capabilities.

The free plan should be useful indefinitely for ordinary participation.

---

# 5. Plus

Purpose:

> More continuity and more capacity.

Potential additions:

* larger Story;
* additional storage;
* enhanced personalization;
* richer Circle features;
* advanced sharing;
* enhanced Possibility exploration;
* expanded history.

---

# 6. Advanced

Purpose:

> More power for serious personal or organizational use.

Potential additions:

* advanced intelligence;
* automation;
* richer reporting;
* advanced Vault;
* advanced Projects;
* integrations;
* larger capacity;
* advanced permissions.

---

# 7. Organization

Purpose:

> Operational infrastructure for larger Circles.

Potential additions:

* advanced administration;
* large membership;
* audit tools;
* governance;
* advanced finance;
* APIs;
* enterprise integrations;
* dedicated support;
* advanced security controls.

---

# 8. Archetype Adaptation

The same plan should not necessarily expose the same features everywhere.

Example:

### Family Plus

May emphasize:

* Story;
* continuity;
* Vault;
* property;
* succession.

### Business Plus

May emphasize:

* projects;
* invoices;
* inventory;
* payments;
* staff.

### Community Plus

May emphasize:

* governance;
* treasury;
* membership;
* projects.

The plan is the commercial layer.

The archetype determines the experience.

---

# 9. Capacity

Every plan has capacity dimensions.

Potential dimensions:

```text
PEOPLE
CIRCLES
STORAGE
STORY
PROJECTS
TRANSACTIONS
AUTOMATION
AI USAGE
INTEGRATIONS
API USAGE
```

Not all dimensions need to be exposed to users.

---

# 10. Capacity Language

Avoid technical language like:

> “You have consumed 84% of your relational-object quota.”

Instead:

> **You've used most of your included Circle capacity.**

Then explain what specifically is approaching its limit.

---

# 11. Capacity Bands

The billable unit is **Circle Capacity Units (CCU)** — decided in ADR-0003. It is
never a person, never an account, and never an identifier from the namespace.

A Circle's CCU consumption is metered across:

* active seats;
* storage;
* media / ingest volume;
* AI / compute usage;
* transactions processed;
* API calls;
* connected integrations.

Bands exist only as an internal escalation ladder for the ORGANIZATION tier:

```text
Band 1  small   — included allowance
Band 2  medium  — +1 step
Band 3  large   — +2 steps
...
Band N  capped  — beyond this, custom terms
```

Personal, Family and Fellowship contexts are **zero-price** with a generous fixed
allowance (ADR-0003), so most users will never see a band at all.

---

# 12. Price Composition (ADR-0004)

Pricing is **not** a curve that escalates with platform size. A price that doubles
per user milestone was explicitly rejected: it contradicts §13, contradicts §36,
and would charge a three-person Family Circle for other people's growth.

The engine composes a price instead of scaling one:

```text
PLAN BASE PRICE      published, predictable, per plan (§3), per currency (§25)
+ METERED ADD-ONS    CCU dimensions above the included allowance (§15)
+ BUNDLE             where capabilities naturally work together (§16)
- RULES              sponsorship, nonprofit, education (§35)
= PRICE
```

Where an ORGANIZATION tier escalates by band, escalation is **linear and capped**,
and subject to the rate cap in §13. Monetary values come from the cost model in
[Monetization & Economics.md](Monetization%20&%20Economics.md), anchored to the
only price points already tested in this company (CONUS: 100 FCFA daily /
1,000 FCFA monthly).

---

# 13. Cap and Rate Cap

Two different guarantees, both required.

**Absolute cap (ORGANIZATION only).** A maximum subscription price exists; above
it the relationship moves to custom terms.

> **Your price will never exceed this cap.**

**Rate cap (every plan).** A customer's price may escalate by **at most one band
per billing year**, with advance notice and grandfathering (§29, §33).

> **Your bill cannot jump more than one step in a year.**

The rate cap is the promise that actually protects a customer, because it is
verifiable against their own invoices. The absolute cap only bounds the worst
case; the rate cap bounds the surprise.

---

# 14. The Cap and Fair Use

A capped price should not mean unlimited abuse.

Circle can still enforce:

* anti-abuse limits;
* security limits;
* reasonable-use policies;
* storage restrictions;
* prohibited activity controls.

---

# 15. Capability Add-ons

Some capabilities may be separately priced.

Examples:

* advanced Cargo;
* high-volume Inventory;
* specialized financial integrations;
* advanced API;
* high-compute AI.

But Circle should avoid turning every feature into an add-on.

---

# 16. Bundle Strategy

Where several capabilities naturally work together, bundle them.

Example:

> **Business Operations**

could include:

* Projects;
* Inventory;
* Orders;
* Payments;
* Reporting.

This is easier to understand than five separate micro-purchases.

---

# 17. Billing Period

Support:

* monthly;
* annual.

Annual pricing may offer an incentive without making monthly access artificially difficult.

---

# 18. Payment Failure

A failed payment should not immediately destroy access.

Suggested flow:

```text
PAYMENT FAILED
↓
PAST_DUE          (automatic retries)
↓
GRACE             (time-boxed, warned)
↓
RESTRICTED        (Limited Mode — the user-facing label)
↓
INACTIVE          (commercial only — data intact)
↓
CLOSED            (billing relationship ends — data still intact)
```

State names are canonical per ADR-0001. `RETRY` is a mechanism inside `PAST_DUE`,
not a state. Free accounts never enter this chain.

---

# 19. Limited Mode

> **Limited Mode** is the user-facing label for the `RESTRICTED` subscription
> state (ADR-0001). Code and telemetry use `RESTRICTED`; copy uses "Limited Mode".

When subscription expires, Circle should preserve essential continuity.

Potentially:

* view existing data;
* communicate with existing members;
* export data;
* renew;
* reactivate.

Creation of capacity-intensive objects may be restricted.

---

# 20. No Dark Patterns

The pricing interface should not:

* hide cancellation;
* manipulate urgency;
* make free users feel inferior;
* repeatedly interrupt basic use;
* make downgrade difficult.

---

# 21. Upgrade Moments

Upgrade prompts should appear when the user has encountered a genuine boundary.

Example:

> “Your Business has reached the project capacity included in your plan.”

Then:

> **Upgrade to continue creating projects.**

Not:

> “Unlock your full potential!!!”

---

# 22. Downgrade

Downgrading should explain consequences before confirmation.

Example:

> “Your current Circle contains 240 active members. The Free plan supports 100.”

Then offer:

* retain existing data in limited mode;
* archive excess;
* export;
* cancel.

---

# 23. Cancellation

Cancellation should be straightforward.

The user should understand:

* when billing ends;
* what remains accessible;
* when restrictions begin;
* when deletion occurs;
* whether reactivation is possible.

---

# 24. Regional Pricing

Circle should eventually support regional economics.

Potential factors:

* local purchasing power;
* payment infrastructure;
* taxes;
* currency;
* provider fees.

Regional pricing must remain transparent.

---

# 25. Currency Display

Prices should show:

* currency;
* billing frequency;
* taxes where applicable;
* renewal amount.

Do not surprise users at checkout.

---

# 26. Taxes

The pricing system must support jurisdiction-specific taxes.

This belongs in the billing infrastructure rather than hard-coded into plan definitions.

---

# 27a. Personal Context Pricing

The individual context is **not** an archetype (ADR-0002), and it still needs a
defined price, because a person is the largest cohort in the product.

A Person with a Personal Face pays **nothing, permanently**, for:

* their Face and visibility controls;
* Story, Now, Next and Possibilities;
* basic communication and search;
* belonging to any number of Circles.

A person may optionally subscribe to `PLUS` / `ADVANCED` for personal capacity —
storage, history depth, advanced intelligence, privacy and continuity controls.
That is a self-upgrade, never a requirement to participate.

Individuals are never billed for belonging to a Circle; that is billed to the
Circle's billing account (PRD §79, ADR-0003).

---

# 27. Business Pricing

Business pricing depends on measured CCU consumption (ADR-0003), not on headcount
alone. It may eventually depend on:

* active members;
* operational capacity;
* capabilities;
* transaction volume;
* storage;
* integrations.

Avoid charging simply because a person has a Business Face.

---

# 28. Family Pricing

Family pricing should avoid penalizing large families merely because they are large.

If paid capacity becomes necessary, pricing should emphasize:

* shared infrastructure;
* storage;
* advanced features;
* continuity.

Basic kinship should remain free.

---

# 29. Fellowship Pricing

Fellowships should be lightweight.

A small group should not need a corporate subscription simply to:

* plan a trip;
* share expenses;
* preserve memories.

---

# 30. Community Pricing

Community pricing should account for the difference between:

* volunteer community;
* local organization;
* large institution.

The commercial model should scale with actual infrastructure requirements.

---

# 31. Pricing Transparency

The user should always be able to answer:

> What am I paying?

> Why am I paying it?

> What happens if I stop?

> What happens to my data?

> Can I come back?

---

# 32. Pricing API

Internally:

```text
getPrice(
    archetype,
    plan,
    capacity,
    capabilities,
    region,
    billingPeriod
)
```

should return a deterministic pricing result.

The pricing engine must be versioned.

---

# 33. Pricing Versioning

A subscription should retain:

* pricing version;
* plan;
* capacity band;
* discounts;
* taxes;
* billing period.

This allows historical billing to remain understandable even after pricing changes.

---

# 34. Promotional Pricing

Promotions should have:

* start;
* end;
* eligibility;
* discount;
* renewal behavior.

Users must know whether the discount continues after the promotional period.

---

# 35. Scholarships / Sponsored Access

Circle may support:

* sponsored subscriptions;
* nonprofit pricing;
* community grants;
* educational access.

These should be implemented as billing rules rather than special product forks.

---

# 36. Pricing UX

The ideal pricing page should be explainable in a few sentences.

Something like:

> **Circle is free for everyday life.**
>
> When you need more capacity or advanced capabilities, you can upgrade.
>
> Businesses and larger Circles can pay for the infrastructure they use.
>
> There is a maximum price.
>
> And your price cannot jump by more than one step in a year.

That is the emotional simplicity the system should preserve. The last line is the
one a customer can verify against their own invoices — which is why ADR-0004
keeps the rate cap alongside the price cap.

---

# 37. Final Principle

The user should think:

> **“I understand what Circle costs.”**

not:

> **“I need to study Circle's pricing system.”**
