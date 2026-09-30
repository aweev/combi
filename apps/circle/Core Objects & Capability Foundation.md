# CIRCLE

## Core Objects & Capability Foundation

### The smallest set of primitives from which Circle can grow

---

# 1. Purpose

Circle should not become a collection of unrelated features.

Every feature should be built from a small set of powerful primitives.

The goal is:

> **few primitives, many possibilities.**

---

# 2. The Core Object Model

The foundational objects are:

```text
PERSON
CIRCLE
RELATIONSHIP
STORY
ACTIVITY
COMMITMENT
EVENT
PROJECT
OBJECT
DOCUMENT
MEDIA
TRANSACTION
CAPABILITY
```

Not every object needs to be visible as a separate product concept.

---

# 3. Person

Represents an individual identity.

A Person can:

* belong to Circles;
* have relationships;
* create Story;
* participate in Activities;
* make Commitments;
* own or access Objects;
* generate Possibilities.

---

# 4. Circle

Represents a collective context.

A Circle contains:

* people;
* relationships;
* shared Story;
* Now;
* Next;
* Possibilities;
* capabilities.

---

# 5. Relationship

Connects two entities.

Examples:

```text
Person → Person
Person → Circle
Circle → Circle
Person → Business
Business → Community
```

Relationships can have:

* type;
* role;
* status;
* start date;
* end date;
* visibility;
* provenance.

---

# 6. Story

Represents something worth preserving.

Story can originate from:

* Activity;
* Event;
* Project;
* Commitment;
* Transaction;
* manually created memory.

Story preserves meaning over time.

---

# 7. Activity

Represents something happening.

Examples:

* message;
* contribution;
* visit;
* upload;
* update;
* transaction;
* participation.

Not every Activity becomes Story.

---

# 8. Commitment

Represents an intended responsibility or promise.

Examples:

> "I will call Daniel tomorrow."

> "Business will deliver by Friday."

> "Members will contribute 20,000 each."

Commitments can belong to individuals or Circles.

---

# 9. Event

Represents something scheduled or occurring at a specific time.

Examples:

* birthday;
* meeting;
* wedding;
* trip;
* launch;
* community event.

Events can generate Story.

---

# 10. Project

Represents coordinated work toward an outcome.

A Project may contain:

* people;
* tasks;
* milestones;
* documents;
* expenses;
* commitments;
* events;
* Story.

---

# 11. Object

Object is a deliberately broad primitive.

It can represent:

* property;
* product;
* asset;
* resource;
* inventory item;
* location;
* equipment.

The exact capabilities determine how it behaves.

---

# 12. Document

Represents structured or uploaded information.

Examples:

* contract;
* receipt;
* certificate;
* agreement;
* report.

Documents can be attached to almost any contextual object.

---

# 13. Media

Represents:

* photo;
* video;
* audio.

Media can become Story or evidence.

---

# 14. Transaction

Represents a financial event.

It connects:

* people;
* Circles;
* amounts;
* purposes;
* evidence;
* obligations.

---

# 15. Capability

A Capability is what a Circle is able to do.

Examples:

```text
MESSAGING
STORY
EVENTS
PROJECTS
VAULT
INVENTORY
ORDERS
BOOKING
GOVERNANCE
MEMBERSHIP
DOCUMENTS
```

Capabilities are modular.

---

# 16. Capability ≠ Feature

A feature is something the user sees.

A capability is an underlying ability.

Example:

> "Create an invoice"

may depend on:

```text
VAULT
+
DOCUMENTS
+
RELATIONSHIPS
+
TRANSACTIONS
```

This keeps the architecture reusable.

---

# 17. Capability Composition

Capabilities can combine.

Example:

```text
BUSINESS
+
PROJECTS
+
VAULT
+
DOCUMENTS
+
RELATIONSHIPS
=
PROJECT-BASED BUSINESS OPERATIONS
```

No new application is necessary.

---

# 18. Capability Dependencies

Capabilities may have dependencies.

Example:

```text
INVOICING
   ↓
VAULT
   ↓
RELATIONSHIPS
```

Circle should manage these dependencies automatically.

---

# 19. Capability Activation

A capability can be:

```text
AVAILABLE
ENABLED
DISABLED
RESTRICTED
DEPRECATED
```

Existing data should remain safe when a capability is disabled.

---

# 20. Capability Discovery

Users should discover capabilities through needs.

Example:

> "We need to collect contributions."

Circle can suggest:

> **Enable Vault**

rather than presenting a giant catalog of 80 modules.

---

# 21. Progressive Disclosure

The interface should initially show:

> what matters now.

Advanced functionality appears when relevant.

This protects Circle's simplicity.

---

# 22. The Universal Context

Every object should be capable of belonging to a context.

Conceptually:

```text
OBJECT
 ↓
CONTEXT
 ↓
PEOPLE
 ↓
CIRCLE
 ↓
TIME
```

This allows Circle to answer:

> "Why am I seeing this?"

---

# 23. Universal Timeline

Different objects can contribute events to a timeline.

Example:

```text
PROJECT CREATED
PAYMENT RECEIVED
DOCUMENT UPLOADED
MEETING HELD
PROJECT COMPLETED
STORY CREATED
```

The timeline should remain contextual rather than becoming one giant global feed.

---

# 24. Universal Activity

The Activity system records what happened.

It can support:

* audit;
* notifications;
* Now;
* analytics;
* Story candidates.

Activity is operational.

Story is historical.

This distinction is important.

---

# 25. Universal State

Objects should have explicit state.

Examples:

```text
DRAFT
ACTIVE
PENDING
COMPLETED
ARCHIVED
CANCELLED
DISPUTED
```

Domain-specific objects can have additional states.

---

# 26. Universal Ownership

Circle should distinguish:

* created by;
* owned by;
* managed by;
* associated with;
* visible to.

These are not the same relationship.

---

# 27. Universal Evidence

Evidence can attach to:

* Story;
* Transaction;
* Project;
* Commitment;
* Relationship;
* Event.

This creates continuity across modules.

---

# 28. Universal Permissions

Permissions should operate consistently across capabilities.

Conceptually:

```text
IDENTITY
↓
CIRCLE MEMBERSHIP
↓
ROLE
↓
CAPABILITY
↓
OBJECT
↓
ACTION
```

Example:

> A person may belong to a Business but cannot approve payments.

---

# 29. Universal Search

Search should operate across objects.

A user could search:

> "Water project"

and find:

* Project;
* Story;
* Documents;
* Payments;
* People;
* Events;
* Possibilities.

Search is therefore contextual, not merely textual.

---

# 30. Universal Notifications

Notifications should be generated from meaningful state changes.

Examples:

* invitation;
* commitment;
* payment;
* event;
* approval;
* mention;
* relationship request.

Notifications should always retain context.

---

# 31. Universal Sharing

Sharing should reference an existing object rather than creating unnecessary copies.

Example:

> Share Story

does not create a second Story.

It creates another authorized view of the same object.

---

# 32. Universal References

Objects can reference other objects.

Example:

```text
Invoice
→ Project
→ Client
→ Business
→ Transaction
→ Document
```

This is the foundation of Circle's interconnectedness.

---

# 33. Universal History

Objects should preserve important changes.

Example:

```text
Project
Created
→ Assigned
→ Updated
→ Completed
→ Archived
```

History provides accountability and continuity.

---

# 34. Specialized Domains

Specialized systems should sit above the foundation.

For example:

```text
FOUNDATION
│
├── Inventory
├── Cargo
├── Booking
├── Governance
├── Finance
├── Events
└── Commerce
```

These should consume the same:

* People;
* Circles;
* Relationships;
* Objects;
* Transactions;
* Story;
* permissions.

---

# 35. Why Cargo Does Not Need to Be Core

Cargo may be extremely useful for certain businesses.

But it should not become a foundational concept unless Circle discovers that:

> logistics relationships are fundamental to almost every Circle.

Instead:

```text
Business
+
Cargo capability
```

creates a logistics-enabled Business.

This is exactly how Circle should expand.

---

# 36. Inventory

Likewise:

> Inventory is a capability.

It does not require a separate identity model.

An inventory item is simply an Object with inventory-specific behavior.

---

# 37. Commerce

Commerce can compose:

```text
People
+
Products
+
Relationships
+
Orders
+
Transactions
+
Documents
```

Again, no separate platform is necessary.

---

# 38. Governance

Governance can compose:

```text
People
+
Roles
+
Commitments
+
Events
+
Decisions
+
Documents
```

This can support:

* families;
* communities;
* businesses;
* associations.

---

# 39. Decision

A Decision may eventually become a first-class object.

It can record:

* proposal;
* participants;
* options;
* decision;
* date;
* authority;
* evidence;
* outcome.

This is especially valuable for Business, Family, and Community Circles.

---

# 40. Possibility

Possibility sits above the object graph.

It can connect:

```text
CURRENT CONTEXT
+
RELATIONSHIPS
+
CAPABILITIES
+
HISTORY
=
POSSIBILITY
```

Possibility is therefore not merely another database record.

It is a system interpretation that should retain provenance.

---

# 41. The Expansion Rule

Whenever a new feature is proposed:

### Step 1

Can existing objects represent it?

### Step 2

Can existing relationships represent it?

### Step 3

Can an existing capability support it?

### Step 4

Does it require a new primitive?

Only if Step 4 is genuinely necessary should Circle expand its foundational model.

---

# 42. Anti-Bloat Rule

Never create:

> "Circle for X"

when the real requirement is:

> "Circle with capability X."

This is one of the most important architectural constraints.

---

# 43. The Simplicity Test

A new capability should ideally be explainable as:

> **One new ability using existing Circle relationships and objects.**

If it requires:

* new identity;
* new account type;
* new permission system;
* new database model;
* new navigation;
* new billing architecture;

then the team should first question whether it belongs in the foundation.

---

# 44. The Product Test

Every feature must answer:

> Who is this for?

> What problem does it solve?

> What existing context does it connect to?

> What new context does it create?

> Can it remain invisible until needed?

---

# 45. The Architecture Test

Every new domain should ideally fit:

```text
PERSON
+
CIRCLE
+
RELATIONSHIP
+
OBJECT
+
ACTIVITY
+
STORY
+
NEXT
+
CAPABILITY
```

If it fits naturally, Circle is successfully becoming a platform rather than a bundle of products.

---

# 46. Final Foundation

The deepest Circle architecture can therefore be reduced to:

```text
                  PEOPLE
                     │
              RELATIONSHIPS
                     │
                   CIRCLES
                     │
          ┌──────────┼──────────┐
          ↓          ↓          ↓
       OBJECTS    ACTIVITIES   EVENTS
          │          │          │
          └──────────┼──────────┘
                     ↓
                 CONTEXT
                     │
              ┌──────┴──────┐
              ↓             ↓
            STORY         NOW
              │             │
              └──────┬──────┘
                     ↓
                    NEXT
                     ↓
               POSSIBILITY
                     ↓
               CAPABILITIES
                     ↓
                NEW ACTION
```

---

# 47. Final Principle

> **Circle should grow by composing capabilities, not by multiplying products.**

That principle protects the project from becoming complicated while giving it enormous room to expand.
