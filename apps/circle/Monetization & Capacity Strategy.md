# CIRCLE

## Monetization & Capacity Strategy

### A mostly-free product with scalable participation economics

---

# 1. Purpose

Circle should be financially sustainable without making its core human value feel artificially paywalled.

The central principle is:

> **Circle should be mostly free to belong to.**

People should be able to create meaningful relationships, participate in Circles, preserve Story, understand Now, plan Next, and discover Possibilities without feeling that basic human participation requires a subscription.

Revenue should primarily come from:

* scale;
* advanced capabilities;
* organizational needs;
* higher capacity;
* specialized infrastructure;
* optional upgrades.

---

# 2. The Economic Philosophy

Circle is intended to become more valuable as more people and Circles exist within it.

Therefore the business model should align with:

> **more value → more participation → sustainable capacity pricing**

rather than:

> **basic human participation → recurring paywall**

---

# 3. Primary Pricing Unit (decided — ADR-0003)

The commercial unit is **Circle Capacity Units (CCU)**: the capacity a Circle
consumes, metered across active seats, storage, media/ingest, AI/compute,
transactions, API calls and integrations.

It is **not**:

* a person;
* an account;
* an identifier from the name space (namespace scarcity is a technical concern and
  must never be monetised — see §24 and ADR-0005).

The billed party is the Circle's billing account, never an individual who merely
belongs. Archetypes still differ economically: Personal, Family and Fellowship
contexts are zero-price with a generous fixed allowance; Business and large
Community are the metered segments (§20).

---

# 4. Archetype Economics

Each archetype may eventually have its own pricing model. There are exactly four
archetypes (ADR-0002): Family, Fellowship, Business, Community.

### Personal (context, not an archetype)

Primarily free.

Optional upgrades for advanced:

* storage;
* intelligence;
* privacy;
* continuity;
* integrations;
* specialized capabilities.

### Family

Primarily free.

Optional upgrades for:

* larger family capacity;
* Vault;
* property;
* advanced continuity;
* shared storage;
* succession;
* advanced family administration.

### Fellowship

Primarily free.

Optional upgrades for:

* larger groups;
* advanced coordination;
* shared Vault;
* projects;
* events;
* storage.

### Business

The strongest candidate for paid plans.

Possible paid capabilities:

* advanced operations;
* finance;
* inventory;
* projects;
* integrations;
* staff administration;
* automation;
* reporting;
* higher storage;
* specialized capabilities.

### Community

Primarily free at basic level.

Optional upgrades for:

* larger membership;
* governance;
* treasury;
* reporting;
* administration;
* advanced infrastructure.

---

# 5. The Free Core

The following should remain fundamental to Circle:

* account creation;
* basic identity;
* basic Face;
* joining Circles;
* basic relationships;
* basic Story;
* basic Now;
* basic Next;
* basic Possibilities;
* basic communication;
* basic participation.

Circle should resist turning these into artificial upsells.

---

# 6. What People Pay For

The product should generally charge for:

### Capacity

More:

* people;
* storage;
* history;
* activity;
* projects;
* transactions.

### Power

More:

* automation;
* intelligence;
* advanced workflows;
* analytics.

### Infrastructure

More:

* integrations;
* API access;
* advanced exports;
* enterprise administration.

### Specialized Capabilities

Examples:

* advanced Cargo;
* advanced Inventory;
* advanced Finance;
* advanced Governance;
* advanced Booking.

---

# 7. Progressive Economics

The initial experience should not require understanding pricing architecture.

A user should encounter:

> **Free**

and only later encounter:

> **You are approaching the capacity included in your current plan.**

This keeps the product human-first.

---

# 8. Capacity Ladder (revised — ADR-0004)

Pricing does **not** escalate with platform-wide user milestones. A price that
doubles per million users was rejected: at a $5 base it reaches $2,560/month by
10M users, which contradicts §10 (a cap that should mean something) and the
Pricing specification's §36 (a customer who understands the price). It also
charges a three-person Family Circle for other people's growth.

What replaces it:

```text
PLAN BASE PRICE      published, predictable
+ METERED ADD-ONS    CCU above the included allowance
+ RATE CAP           at most ONE band of escalation per billing year
+ ABSOLUTE CAP       ORGANIZATION only; above it, custom terms
```

Where band escalation applies (ORGANIZATION), it is **linear**, never exponential.

The important principle survives, with the unit corrected:

> **capacity becomes progressively more expensive as a Circle's own consumption
> scales.**

The unit of escalation is the customer's usage, not the platform's success.

---

# 9. Why Progressive Pricing

This creates a relationship between:

* Circle's scale;
* infrastructure burden;
* commercial value;
* sustainability.

It also allows the product to remain inexpensive at smaller scales.

---

# 10. Pricing Cap and Rate Cap

Two guarantees, both required.

**Absolute cap.** Once an account reaches the maximum intended pricing level:

> additional scale does not create unlimited subscription cost.

**Rate cap (ADR-0004).** Independently of the absolute cap, a customer's bill may
escalate by **at most one band per billing year**, with notice and grandfathering
(§29). The rate cap is the promise a customer can verify against their own
invoices; the absolute cap only bounds the worst case.

The exact cap should be determined through:

* infrastructure economics;
* market testing;
* operating costs;
* payment processing;
* support requirements.

---

# 11. Cap Is Not the End of Capacity

Reaching the pricing cap does not necessarily mean unlimited technical resources.

It means:

> the commercial price is capped.

Technical capacity may still have:

* fair-use limits;
* infrastructure limits;
* security controls;
* abuse protections.

---

# 12. Archetype-Specific Pricing

The pricing engine should support:

```text
ARCHETYPE
+
CAPACITY
+
CAPABILITIES
+
PLAN
+
USAGE
=
PRICE
```

Therefore Business and Family do not need identical pricing.

---

# 13. Capability Pricing

Some capabilities may carry additional costs.

Examples:

* advanced AI;
* high-volume storage;
* external financial integrations;
* specialized logistics;
* advanced reporting;
* API usage.

These should be optional wherever possible.

---

# 14. Avoid Feature Fragmentation

Circle should avoid:

> “You need Premium to create a basic relationship.”

or:

> “You need Pro to preserve a family memory.”

Paid functionality should represent genuine additional infrastructure or value.

---

# 15. Organizational Value

Businesses and organizations can reasonably pay more because Circle can replace or consolidate multiple systems.

Potential value areas:

* CRM;
* project coordination;
* operational records;
* payments;
* documentation;
* governance;
* communication.

The product should earn that value rather than simply charge for access.

---

# 16. Sponsorship / Subsidization

A future model may allow:

* businesses;
* institutions;
* foundations;
* communities

to subsidize access for others.

For example:

> Business subscription revenue helps maintain free community infrastructure.

This should remain transparent.

---

# 17. No Selling Personal Data

Circle's business model should not depend on selling personal data.

Core principle:

> **The user's relationships are not the product being sold to advertisers.**

Revenue should come from Circle's services and infrastructure.

---

# 18. Advertising

Advertising should not be the default economic foundation.

If advertising ever exists, it should be:

* clearly separated;
* privacy-conscious;
* non-invasive;
* optional where feasible;
* incapable of determining what Circle surfaces as truth.

---

# 19. AI Economics

AI costs can become substantial at scale.

Circle should therefore distinguish:

### Basic intelligence

Included for ordinary use.

### Advanced intelligence

May be limited or paid when it requires significant computation.

The product should communicate this through capacity rather than arbitrary intelligence gates.

---

# 20. Economic Fairness

The pricing system should avoid penalizing:

* large families;
* large fellowships;
* community participation.

A Circle with 200 family members should not automatically be treated like a 200-person commercial organization.

Context matters.

---

# 21. Payment Flexibility

The system should support payment methods appropriate to each market.

For African markets this should include consideration of:

* mobile money;
* cards;
* bank transfers;
* local payment providers.

The payment architecture should remain provider-agnostic.

---

# 22. Grace Periods

Payment failures should not immediately destroy a Circle's continuity.

Possible lifecycle:

```text
PAYMENT DUE
↓
PAST_DUE
↓
GRACE
↓
RESTRICTED      (Limited Mode — data intact)
↓
INACTIVE        (commercial only — data intact)
↓
CLOSED          (billing relationship ends — data still intact)
```

State names are canonical per ADR-0001; `ARCHIVED` is a **data** state and must
not appear in the subscription machine.

Critical data should not disappear merely because a payment failed — and per
ADR-0005 it now provably cannot: deletion requires an explicit user request or a
legal obligation.

---

# 23. The Three-Month Rule (revised — ADR-0005)

The old principle — three months of inactivity removes the account and releases
its number — is **withdrawn**. It contradicted this document's own §22, the
Pricing specification's §19 Limited Mode, and the product's North Star of
continuity, and it punished the ordinary dormancy of human life (trips end,
weddings pass, projects finish).

The replacement principle:

> **Three months of inactive subscription ends the commercial relationship, not
> the person's history.**

What may happen after the grace period: the paid plan lapses to `RESTRICTED` then
`CLOSED`; expensive services scale down (cold-storage tiering, suspended
background processing); the free core continues to work.

What must not happen: data deletion, identifier reclamation, or being made to
queue to get your own history back.

The commercial state and the data lifecycle remain separate — that separation is
now enforced rather than merely asserted.

---

# 24. Account Number Capacity (revised — ADR-0003, ADR-0005)

Namespace scarcity is a technical capacity-planning problem and **never** a
billing mechanism. Dormant accounts are not removed to free numbers for other
people.

If the identity namespace genuinely becomes scarce, the correct response is to
expand it — more digits, more prefixes, or an allocation scheme with far more
headroom — not to delete a person's accumulated relationships and recycle their
identifier underneath them. Reusing a human identifier that once belonged to
someone's family record is a permanent source of mis-association bugs and privacy
incidents.

```text
Namespace under pressure
      ↓
expand the namespace / change allocation
      ↓
never reclaim an identifier from living history
```

---

# 25. Returning User (revised — ADR-0005)

A returning user finds their history intact and reactivates through identity
verification (`Lifecycle & Account Status.md` §37).

* Free plan: nothing ended; they simply return.
* Paid plan lapsed to `CLOSED`: a **new subscription** starts against the same
  account and the same data.

There is no reassignment case, because identifiers are not reclaimed (§24).

---

# 26. Long List / Re-entry (deleted — ADR-0005)

The waitlist mechanism is removed. It appeared in this one document only, was
referenced by no other spec, and rested on premises the project has now rejected:
that human identity namespaces should be rationed for commercial capacity (§24,
ADR-0003) and that dormancy may destroy continuity (ADR-0005).

Making a lapsed customer queue to regain their own family history converts a
recoverable churn event into a public critic. There is no successor mechanism;
reactivation is defined in `Lifecycle & Account Status.md` §37.

---

# 27. The Important Distinction

Circle must distinguish:

### Subscription inactivity

The commercial relationship is inactive.

### Account inactivity

The account has not been used.

### Circle inactivity

A particular Circle is inactive.

### Data deletion

Information is actually removed.

These should never be treated as synonyms.

---

# 28. Pricing Architecture

The pricing engine should be configurable rather than hard-coded.

Conceptually:

```text
Pricing Rule
├── Archetype
├── Plan
├── Capacity Band
├── Included Capabilities
├── Additional Capabilities
├── Billing Period
├── Currency
├── Regional Rule
├── Grace Period
├── Maximum Price          (absolute cap — ORGANIZATION)
├── Rate Cap               (one band per billing year — every plan, ADR-0004)
└── Metered CCU Allowance  (ADR-0003)
```

---

# 29. Pricing Changes

Pricing changes should not silently alter existing commitments.

The system should support:

* grandfathering;
* notice periods;
* migration;
* plan conversion;
* cancellation;
* prorating where appropriate.

---

# 30. Economic North Star

Circle should aim for:

> **Millions can participate freely. Organizations that derive substantial operational value can fund the infrastructure.**

That allows Circle to grow as a social platform without making its core social fabric transactional.
