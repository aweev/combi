# CIRCLE

## Archetypes & Capabilities

### A composable foundation for different kinds of Circles

---

# 1. Purpose

Circle must support very different forms of human organization without becoming four separate products.

The solution is:

> **Archetypes provide context. Capabilities provide specialization.**

---

# 2. The Four Initial Archetypes

```text
                 CIRCLE
                    │
        ┌───────────┼───────────┐
        │           │           │
     FAMILY     FELLOWSHIP   BUSINESS
                    │
                COMMUNITY
```

More precisely, all four are peers:

```text
Family
Fellowship
Business
Community
```

No archetype is the parent of another.

---

# 3. Family

## Purpose

Support continuity, care, kinship, memory, shared responsibility, and succession.

### Default concepts

* members;
* kinship;
* guardianship;
* family Story;
* milestones;
* care;
* shared resources;
* succession.

### Common capabilities

* Family Tree;
* Shared Vault;
* Dependent Wallets;
* Events;
* Care;
* Property;
* Documents;
* Succession;
* Memory.

---

# 4. Fellowship

## Purpose

Support meaningful friendship, shared identity, experience, mutual support, and belonging.

### Default concepts

* members;
* friendship;
* shared experiences;
* events;
* commitments;
* memories.

### Common capabilities

* Trips;
* Shared Expenses;
* IOUs;
* Events;
* Projects;
* Shared Goals;
* Memory;
* Contributions.

Fellowship deliberately avoids unnecessary organizational bureaucracy.

---

# 5. Business

## Purpose

Support people working together to create economic value.

### Default concepts

* ownership;
* staff;
* clients;
* suppliers;
* work;
* commitments;
* money;
* evidence;
* operations.

### Common capabilities

* Projects;
* Inventory;
* Orders;
* Invoicing;
* Payments;
* Bookings;
* Logistics;
* Cargo;
* Property;
* Membership;
* Subscriptions;
* Procurement.

The Business archetype itself remains generic.

---

# 6. Community

## Purpose

Support collective identity, governance, shared resources, and collective action.

### Default concepts

* membership;
* representation;
* governance;
* proposals;
* decisions;
* shared resources;
* community Story.

### Common capabilities

* Voting;
* Proposals;
* Treasury;
* Contributions;
* Projects;
* Property;
* Events;
* Notices;
* Governance;
* Membership.

---

# 7. Archetypes Are Defaults

Creating a Business should not activate every Business feature.

Creating a Family should not activate every Family capability.

Instead:

```text
CREATE CIRCLE
      ↓
CHOOSE CONTEXT
      ↓
ARCHETYPE DEFAULTS
      ↓
ENABLE CAPABILITIES AS NEEDED
```

This keeps onboarding simple.

---

# 8. Capability Model

A Capability is:

> a reusable system of functionality that can be attached to one or more Circle types.

Examples:

```text
Inventory
Business ✓
Family ✓
Community ✓
Fellowship optional
```

The underlying capability remains the same.

Its language and defaults can change by context.

---

# 9. Capability Categories

## People

* Membership
* Roles
* Delegation
* Invitations

## Relationships

* Kinship
* Friendship
* Client
* Supplier
* Partner
* Affiliation

## Work

* Projects
* Tasks
* Deliverables
* Approvals

## Money

* Vault
* Contributions
* Payments
* Invoices
* Expenses
* Settlements

## Resources

* Inventory
* Property
* Assets
* Equipment

## Coordination

* Events
* Booking
* Scheduling
* Logistics

## Governance

* Voting
* Proposals
* Decisions
* Policies

## Continuity

* Story
* Documents
* Evidence
* Succession

---

# 10. Cargo

Cargo is an example of a capability.

It should not be part of the fundamental Business schema.

Cargo can provide:

* shipment;
* package;
* sender;
* receiver;
* route;
* carrier;
* status;
* proof of delivery;
* payment.

But the underlying Business still uses:

* People;
* Relationships;
* Projects;
* Commitments;
* Money;
* Assets;
* Evidence.

This allows the same foundation to support:

* logistics;
* restaurant;
* construction;
* retail;
* consulting;
* agriculture;
* transport;
* manufacturing.

---

# 11. Capability Composition

A Business may enable:

```text
Business
├── Inventory
├── Orders
├── Payments
├── Cargo
└── Projects
```

Another Business may enable:

```text
Business
├── Bookings
├── Membership
├── Payments
└── Events
```

Same archetype.

Different composition.

---

# 12. Capability Dependencies

Capabilities can depend on other capabilities.

Example:

```text
Cargo
├── requires People
├── requires Places
├── requires Commitments
├── requires Money [optional]
└── requires Evidence
```

Another:

```text
Voting
├── requires Membership
├── requires Roles
├── requires Identity
└── requires Governance
```

Dependencies should be machine-readable.

---

# 13. Capability States

A capability can be:

```text
AVAILABLE
ENABLED
CONFIGURED
SUSPENDED
DISABLED
RETIRED
```

A Circle should not be forced to configure a capability merely because it exists.

---

# 14. Capability UX

The user should not see:

> “Enable Capability Module: Inventory v3.”

They should see:

> **Add inventory**

or:

> **Do you keep track of products or materials?**

The system translates human intent into capability configuration.

---

# 15. Capability Discovery

Capabilities can emerge contextually.

Example:

A Business repeatedly records:

* products;
* quantities;
* deliveries.

Circle may surface:

> “Would you like a simple inventory space for this?”

This is a possibility, not an automatic feature activation.

---

# 16. Cross-Archetype Capabilities

Some capabilities should work across multiple archetypes.

Examples:

### Events

Family / Fellowship / Business / Community

### Projects

Family / Fellowship / Business / Community

### Vault

Family / Fellowship / Business / Community

### Property

Family / Business / Community

### Documents

All

### Story

All

---

# 17. Archetype-Specific Language

The same capability can use different language.

Example:

**Contribution**

Family:

> Family contribution

Fellowship:

> Shared expense

Business:

> Payment

Community:

> Member contribution

The underlying financial model can remain unified.

---

# 18. Archetype-Specific Permissions

Permissions should adapt to context.

Example:

### Family

A parent may manage dependent records.

### Business

A manager may approve expenses.

### Community

A treasurer may manage treasury but not membership.

The permission engine remains unified.

---

# 19. Capability Configuration

Each capability should have:

```text
Capability
├── Identity
├── Version
├── Dependencies
├── Permissions
├── Data model
├── UI surfaces
├── Events
├── Audit requirements
└── Pricing implications
```

---

# 20. Capability Marketplace

A traditional marketplace is probably unnecessary at first.

Circle should instead provide a simple:

> **Add capability**

experience.

Eventually, external developers or specialized providers may contribute capabilities.

If that happens, they must conform to Circle's security, identity, privacy, and domain contracts.

---

# 21. Business Vertical Strategy

Circle should not build:

> “The Circle app for restaurants.”

or:

> “The Circle app for trucking.”

Instead:

```text
Circle
+
Business
+
Capabilities
=
Industry-specific system
```

This is the foundation for vertical expansion without fragmentation.

---

# 22. Capability Examples by Business

### Restaurant

```text
Business
+ Inventory
+ Orders
+ Bookings
+ Staff
+ Payments
```

### Construction

```text
Business
+ Projects
+ Procurement
+ Inventory
+ Property
+ Payments
+ Evidence
```

### Transport

```text
Business
+ Cargo
+ Logistics
+ Fleet / Assets
+ Payments
+ Evidence
```

### Consultancy

```text
Business
+ Projects
+ Clients
+ Documents
+ Invoices
+ Payments
```

The underlying Circle remains the same.

---

# 23. The Capability Rule

Before creating a new archetype ask:

> Can this be represented as a capability?

Before creating a new capability ask:

> Can this be represented through existing capabilities and domain objects?

This prevents architecture inflation.

---

# 24. The Archetype Rule

Before adding a new archetype ask:

1. Does it represent a genuinely different social relationship?
2. Does it require fundamentally different governance?
3. Does it need different privacy assumptions?
4. Does it have a distinct continuity model?
5. Can the difference be handled through capabilities instead?

Only then should a new archetype be considered.

---

# 25. Future Archetypes

Potential future archetypes should not be hard-coded now.

Candidates might eventually include:

* Institution;
* Cooperative;
* School;
* Faith/community organization;
* Household.

But these should first be tested as compositions of existing primitives.

---

# 26. Archetype × Capability Matrix

| Capability |   Family | Fellowship | Business | Community |
| ---------- | -------: | ---------: | -------: | --------: |
| Story      |        ✓ |          ✓ |        ✓ |         ✓ |
| Events     |        ✓ |          ✓ |        ✓ |         ✓ |
| Projects   |        ✓ |          ✓ |        ✓ |         ✓ |
| Vault      |        ✓ |          ✓ |        ✓ |         ✓ |
| Documents  |        ✓ |          ✓ |        ✓ |         ✓ |
| Property   |        ✓ |          — |        ✓ |         ✓ |
| Inventory  | optional |          — |        ✓ |  optional |
| Cargo      |        — |          — | optional |  optional |
| Voting     | optional |   optional | optional |         ✓ |
| Membership |        ✓ |          ✓ |        ✓ |         ✓ |
| Governance | optional |      light | optional |         ✓ |
| Succession |        ✓ |          — | optional |         ✓ |
| Bookings   | optional |          ✓ |        ✓ |         ✓ |

This matrix is a starting architecture, not a fixed product promise.

---

# 27. Core Insight

The most important architectural decision is:

> **Circle should not know every kind of business. It should know how Circles work.**

Specialization happens through composition.

That makes the platform:

* simpler;
* extensible;
* memorable;
* less redundant;
* easier to maintain;
* capable of supporting many real-world use cases.

---

# 28. Final Model

```text
                    CIRCLE
                       │
            ┌──────────┴──────────┐
            │                     │
        ARCHETYPE            CAPABILITIES
            │                     │
     ┌──────┼──────┐       ┌──────┼──────┐
     │      │      │       │      │      │
   Family Fellow Business Community Work Money
                                  │
                         Inventory / Cargo /
                         Booking / Voting /
                         Property / etc.
```

The archetype provides **meaning**.

The capability provides **power**.

The domain model provides **stability**.
