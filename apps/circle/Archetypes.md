# CIRCLE

## Archetypes (merged canonical document)

**Status:** AGREED — supersedes `Archetypes & Circle Types.md` and `Archetypes & Capabilities.md`, whose full content is preserved below and above.
**Merge rationale:** two ~740-line documents defined the same archetype set with different enums (DOC_INDEX 4). ADR-0002 resolved the conflict; this file is the single surviving document.

---

## PART I — Archetypes & Circle Types (canonical)

---

# 1. Purpose

Circle must support many kinds of real-world relationships without creating a separate application for each one.

The core principle is:

> **One foundation. Different configurations.**

A Family, Business, Fellowship, Community, or another collective should use the same underlying Circle system.

---

# 2. What Is an Archetype?

An archetype describes the dominant nature of a Circle.

It influences:

* terminology;
* default capabilities;
* roles;
* permissions;
* workflows;
* suggested setup;
* relevant surfaces.

It does not create an entirely separate product.

---

# 3. Core Archetypes (ADR-0002)

A Circle has exactly four archetypes:

```text
FAMILY
FELLOWSHIP
BUSINESS
COMMUNITY
```

`PERSON` is **not** an archetype. Person is a primitive (`Core Objects &
Capability Foundation.md` §3) and a Circle is a *collective* context
(`Domain Model.md` §6). The individual context is served by Person + Personal
Face + the `ME` navigation surface; the label "Individual / Me" survives only as
navigation, never as a model term.

Additional archetypes can be introduced later.

---

# 4. Person

The Person is the fundamental individual context.

It has:

* Face;
* Story;
* Now;
* Next;
* Possibilities;
* relationships;
* personal space.

The Person does not need to be considered a Circle in the same way as a collective.

---

# 5. Family

A Family Circle is centered around enduring relationships and shared life.

Potential capabilities:

* Family Story;
* events;
* shared Vault;
* family records;
* property;
* important documents;
* family planning;
* succession;
* shared memories.

---

# 6. Fellowship

A Fellowship is a group organized around a shared purpose or experience.

Examples:

* friends;
* travel group;
* study group;
* creative collective;
* sports group;
* temporary project group.

Fellowships should be lightweight.

---

# 7. Business

A Business Circle represents an organization operating toward economic or professional objectives.

Potential capabilities:

* people;
* roles;
* projects;
* clients;
* suppliers;
* products;
* inventory;
* finance;
* documents;
* operations.

---

# 8. Community

A Community Circle represents a collective connected by place, purpose, identity, interest, or shared responsibility.

Potential capabilities:

* membership;
* governance;
* announcements;
* projects;
* contributions;
* events;
* community Story;
* shared resources.

---

# 9. Archetype Is Configuration

The same underlying object can appear differently.

For example:

> **Person**

in a Business may be:

> Employee

in a Family:

> Daughter

in a Fellowship:

> Organizer

The underlying person remains the same.

---

# 10. Archetype + Capabilities

An archetype determines recommended capabilities.

Conceptually:

```text
ARCHETYPE
   ↓
DEFAULT CAPABILITIES
   ↓
OPTIONAL CAPABILITIES
   ↓
ENABLED CIRCLE
```

---

# 11. Capability Examples

Possible capabilities:

```text
STORY
EVENTS
PROJECTS
TASKS
VAULT
INVENTORY
ORDERS
BOOKING
DOCUMENTS
GOVERNANCE
MEMBERSHIP
MESSAGING
MEDIA
```

Not every Circle needs every capability.

---

# 12. Capability Activation

A Circle should be able to activate capabilities when needed.

Example:

A Fellowship starts with:

* Members;
* Events;
* Story.

Later it activates:

* Vault.

Later:

* Projects.

The Circle grows without forcing unnecessary complexity.

---

# 13. Capability Removal

Capabilities can be deactivated when no longer needed.

Deactivation should generally archive rather than destroy associated history.

---

# 14. Mixed-Use Circles

Real life does not always fit categories.

A group may be:

> friends + business partners + community organizers.

Circle should allow a primary archetype with additional capabilities rather than forcing users to choose a perfect category.

---

# 15. Custom Circle

A future option can allow:

> **Custom Circle**

The user starts from the foundation and selects capabilities.

But this should not become the default.

Most people should receive useful defaults.

---

# 16. Archetype Selection

Circle creation can ask:

> **What is this Circle for?**

Possible choices:

* Family;
* Friends / Fellowship;
* Business;
* Community;
* Something else.

This is easier than asking users to understand technical architecture.

---

# 17. Archetype Terminology

The UI should adapt naturally.

Example:

### Business

> Customers

### Family

> Family members

### Fellowship

> Members

### Community

> Participants / Members

The underlying system can still use the same membership model.

---

# 18. Archetype-Specific Home

Every Circle should have a Home.

But its emphasis changes.

### Family Home

```text
Now
Upcoming
Family Story
People
Vault
```

### Business Home

```text
Now
Projects
Customers
Finance
Operations
Story
```

### Fellowship Home

```text
Now
Events
Members
Shared Plans
Story
```

---

# 19. Archetype-Specific Story

The Story system remains universal.

But the content changes.

### Family

> "Grandmother turned 80."

### Business

> "Opened second branch."

### Community

> "Water project completed."

### Fellowship

> "Our trip to Limbe."

Same underlying Story object.

---

# 20. Archetype-Specific Now

Now is contextual.

### Family

> upcoming birthday.

### Business

> overdue invoice.

### Fellowship

> tomorrow's event.

### Community

> pending project decision.

The Now engine remains the same.

---

# 21. Archetype-Specific Next

Likewise:

### Family

> plan reunion.

### Business

> launch product.

### Fellowship

> organize trip.

### Community

> begin project.

---

# 22. Archetype-Specific Possibility

The Possibility Engine can use archetype context.

Business:

> possible customer relationship.

Family:

> possible shared activity.

Fellowship:

> possible event.

Community:

> possible initiative.

The intelligence remains unified.

---

# 23. Archetype Does Not Determine Identity

Choosing:

> Business

does not turn a person into:

> Business User.

It creates a Business context around existing people.

---

# 24. Multiple Circle Memberships

A person can belong to unlimited or plan-limited Circles.

The same identity should be reused.

This prevents:

> one person → one account per organization.

---

# 25. Cross-Circle Identity

One person can connect:

```text
PERSON
│
├── Family
├── Business
├── Fellowship
└── Community
```

Each Circle sees only authorized context.

---

# 26. Circle Relationships

Circles can relate to other Circles.

Examples:

```text
Business A
↕ supplier relationship
Business B

Family
↕ owns
Business

Community
↕ partners with
Business
```

These are relationships between collective entities.

---

# 27. Nesting — Branches and Affiliations (ADR-0008)

Unbounded "circle inside circle" nesting is **not** supported. Two mechanisms,
both already defined in `Architecture.md` §24, cover the real needs:

**Branch** — parent → child, **depth capped at 1**. A branch is a Circle in its
own right and cannot itself have branches.

```text
Community
├── Youth Group
├── Sports Group
└── Project Team

Business
├── Operations
├── Marketing
└── Finance
```

**Affiliation** — peer link between any two Circles, each side explicitly
choosing what it discloses. Use for relationships, not hierarchies:
`Family ── Business`, `Community ── Family`.

Both reuse the existing Circle primitives — `Domain Model.md` §5 already lists
`Circle → Circle` as a relationship type, and a branch adds a single
`parent_circle_id` field — so no new primitive is required (Expansion Rule,
`Core Objects.md` §41).

Rationale: at depth 1 an effective-permission check touches at most one parent,
which keeps permission inheritance, contextual privacy (`Product Philosophy.md`
§19) and lawful subject-access deletion tractable.

---

# 28. Avoiding Organizational Complexity

Nested Circles should only be created when they solve a real problem — and the
depth cap in §27 now enforces that rather than merely advising it. Circle should
not encourage users to create:

> Circle inside Circle inside Circle

merely because the software permits it. A genuinely deep organisation (national
association → region → chapter → team) is served by Affiliation links plus a
Directory capability, not by recursion.

---

# 29. Temporary Circles

Some Circles naturally expire.

Examples:

* wedding planning;
* trip;
* campaign;
* event;
* short-term project.

They should have a natural lifecycle.

---

# 30. Permanent Circles

Others may be long-lived:

* Family;
* Business;
* Community.

Circle should support both without forcing the user to choose between different products.

---

# 31. Archetype Migration

A Circle's purpose can change.

Example:

> Fellowship → Business

or:

> informal group → registered Community.

The system should support controlled migration rather than forcing users to recreate everything.

---

# 32. Archetype and Monetization

Archetype can influence:

* default plan;
* capability recommendations;
* pricing;
* limits.

But:

> **archetype alone should not determine price.**

Actual usage and capabilities matter.

---

# 33. Archetype and Governance

Different archetypes require different default governance.

Example:

### Family

More relationship-based.

### Business

More role-based.

### Community

Potentially more democratic or steward-based.

The foundation supports all three.

---

# 34. Archetype and Trust

The type of Circle can provide context for trust.

A verified Business is different from:

> an informal Fellowship.

But the interface should not turn archetype into a universal trust score.

---

# 35. Future Archetypes

Potential future configurations:

* School;
* Church / Fellowship organization;
* NGO;
* Club;
* Cooperative;
* Household;
* Professional practice;
* Sports team;
* Creative studio;
* Association.

These should be added only when the foundation can support them naturally.

---

# 36. The Cargo Principle

Specialized functionality should be built as a **capability**, not as a completely separate product.

For example:

> Cargo

can be a logistics capability.

It does not need to define Circle's entire architecture.

This preserves the foundational model.

---

# 37. The Foundation Test

Before adding an archetype, ask:

> Can this be represented using existing Circle primitives?

If yes:

> configure the foundation.

If no:

> identify the genuinely missing primitive.

Only then should a new architectural component be created.

---

# 38. Archetype Test

A new archetype should justify itself by requiring meaningfully different:

* relationships;
* permissions;
* terminology;
* workflows;
* lifecycle;
* economics.

Otherwise it should remain a configuration.

---

# 39. Final Model

```text
                  CIRCLE FOUNDATION
                         │
       ┌─────────────────┼─────────────────┐
       ↓                 ↓                 ↓
   ARCHETYPE         CAPABILITIES      RELATIONSHIPS
       │                 │                 │
       └─────────────────┼─────────────────┘
                         ↓
                  CIRCLE EXPERIENCE
```

The user sees a simple Circle.

The architecture underneath remains flexible.

---

# 40. Final Principle

> **Do not build a different product for every kind of life. Build one system that understands different kinds of life.**

That is how Circle can expand without becoming fragmented.

---

## PART II — Capabilities (formerly `Archetypes & Capabilities.md`)

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
