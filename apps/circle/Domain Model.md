# CIRCLE

## Domain Model

### The underlying objects, relationships, states, and continuity model

---

# 1. Purpose

The Domain Model defines the fundamental things that exist inside Circle and how they relate.

The core rule is:

> **Circle should model reality before it models features.**

Instead of creating separate systems for every use case, Circle establishes a small set of durable primitives from which Family, Fellowship, Business, Community, and future capabilities can be composed.

---

# 2. The Core Domain

Circle is built around:

```text
PERSON
  ↓
IDENTITY / FACE
  ↓
RELATIONSHIP
  ↓
CIRCLE
  ↓
PARTICIPATION
  ↓
ACTIVITY / OBJECT
  ↓
STORY / NOW / NEXT
  ↓
POSSIBILITY
```

Supporting these are:

```text
PLACE
TIME
MEDIA
DOCUMENT
MONEY
ROLE
PERMISSION
EVIDENCE
CAPABILITY
```

---

# 3. Person

A **Person** represents the underlying human being.

A Person is distinct from:

* their account;
* their username;
* their Circle membership;
* their Business identity;
* their public profile.

One Person may participate in many contexts.

```text
Person
├── Account
├── Faces
├── Relationships
├── Circle Memberships
├── Story
├── Now
├── Next
└── Possibilities
```

---

# 4. Account

The Account represents access to Circle.

It contains:

* authentication;
* credentials;
* security;
* devices;
* recovery;
* subscription;
* lifecycle state;
* data controls.

The Account is **not the person's identity**.

This distinction allows future support for:

* account recovery;
* multiple Faces;
* delegated access;
* succession;
* account closure.

---

# 5. Face

A **Face** is a context-specific representation of a Person.

Initial Face types:

### Personal Face

Used for personal participation.

### Business Face

Used when acting on behalf of a business.

### Pseudonymous Face

Used when a person intentionally participates without exposing their ordinary identity.

```text
PERSON
│
├── Personal Face
├── Business Face
└── Pseudonymous Face
```

A Face has its own:

* name;
* image;
* description;
* visibility;
* relationships;
* permissions.

But it ultimately belongs to the same underlying Person.

---

# 6. Circle

A Circle is a trusted social or organizational container.

A Circle has:

* identity;
* archetype;
* members;
* roles;
* relationships;
* capabilities;
* activity;
* Story;
* Now;
* Next;
* optional Vault;
* governance;
* settings.

A Circle is not merely a group chat.

---

# 7. Circle Archetype

Every Circle has a primary archetype.

Initial archetypes:

```text
FAMILY
FELLOWSHIP
BUSINESS
COMMUNITY
```

Archetype establishes defaults.

It does not permanently restrict capabilities.

---

# 8. Membership

A Membership connects a Person/Face to a Circle.

```text
Person
  ↓
Face
  ↓
Membership
  ↓
Circle
```

Membership contains:

* status;
* role;
* joined date;
* permissions;
* participation history;
* invitation source;
* departure state.

Possible states:

```text
INVITED
PENDING
ACTIVE
PAUSED
SUSPENDED
LEFT
REMOVED
```

---

# 9. Role

A Role describes what someone is allowed or expected to do inside a context.

Examples:

### Family

* Parent
* Child
* Elder
* Guardian

### Fellowship

* Member
* Organizer

### Business

* Owner
* Admin
* Manager
* Staff
* Contractor

### Community

* Member
* Steward
* Treasurer
* Moderator

Roles are contextual.

A Business Admin does not automatically become a Family Admin.

---

# 10. Relationship

Relationship is a first-class domain object.

It connects people, Faces, Circles, and sometimes other objects.

Examples:

```text
Person A ── sibling ── Person B

Person A ── member ── Circle A

Business A ── supplier ── Business B

Family A ── affiliated with ── Community B
```

Relationships should carry:

* type;
* context;
* state;
* visibility;
* evidence where applicable;
* start/end dates;
* source.

---

# 11. Relationship Context

The same two people can have different relationships.

Example:

```text
Amina
│
├── Sister → Family Circle
├── Friend → Fellowship
├── Employee → Business
└── Treasurer → Community
```

Circle must never collapse these into one generic relationship.

---

# 12. Activity

An Activity records something that happened or is happening.

Examples:

* conversation;
* meeting;
* event;
* contribution;
* work;
* transaction;
* decision;
* upload;
* visit;
* delivery.

Activities can connect to:

* people;
* Circles;
* places;
* time;
* money;
* media;
* commitments.

---

# 13. Object

A domain Object is something that can persist independently.

Examples:

* Person
* Circle
* Project
* Asset
* Document
* Event
* Transaction
* Commitment
* Story
* Goal

An Activity describes something occurring.

An Object describes something that exists.

---

# 14. Story

Story is the durable memory layer.

A Story item may represent:

* a milestone;
* memory;
* decision;
* completed project;
* important event;
* document;
* media;
* evidence;
* historical record.

Story items can reference almost any domain object.

```text
Story
├── People
├── Circle
├── Place
├── Time
├── Activity
├── Media
├── Evidence
└── Related Story
```

---

# 15. Story Is Not an Archive Dump

Not everything becomes Story.

A Story item should have a reason to exist.

It may be:

* deliberately saved;
* automatically eligible for preservation;
* formally recorded;
* historically significant;
* required for accountability.

This distinction protects Story from becoming another infinite activity feed.

---

# 16. Now

Now is a derived view of current state.

It may combine:

* active activities;
* unresolved commitments;
* current membership;
* current projects;
* current balances;
* upcoming events;
* recent meaningful activity.

Now is therefore primarily a **state representation**, not a separate database.

---

# 17. Next

Next represents future-oriented information.

Examples:

* plans;
* goals;
* commitments;
* proposals;
* intentions;
* opportunities;
* possible actions.

Next should distinguish:

### Declared

The person explicitly stated it.

### Agreed

The Circle or another party agreed.

### Suggested

Circle identified it as a possibility.

### Scheduled

It has a confirmed future time.

---

# 18. Possibility

A Possibility is a potential path or connection.

It is not necessarily a task.

Examples:

```text
Person
→ knows photographer
→ launching business
→ photographer works in branding
→ Possible introduction
```

or:

```text
Completed project
→ demonstrates capability
→ similar need exists in another Circle
→ Possible opportunity
```

Possibilities should preserve their origin and confidence/context.

---

# 19. Commitment

A Commitment represents an obligation or promise.

It can connect:

* person;
* Circle;
* object;
* deadline;
* amount;
* evidence;
* status.

States:

```text
PROPOSED
AGREED
ACTIVE
COMPLETED
CANCELLED
DISPUTED
EXPIRED
```

Commitments are important because they bridge:

> Now → Action → Story

---

# 20. Event

An Event represents something occurring at a specific time or range.

Examples:

* birthday;
* meeting;
* funeral;
* wedding;
* trip;
* community gathering;
* business appointment.

Events can become Story.

Events can create Commitments.

Events can generate Relationships.

---

# 21. Project

A Project groups related activity toward an outcome.

Examples:

* Build a house.
* Launch a business.
* Organize a wedding.
* Complete a client project.
* Build a community water system.

Projects can contain:

* people;
* tasks;
* commitments;
* documents;
* money;
* events;
* Story.

---

# 22. Asset

An Asset represents something valuable or meaningful.

Examples:

* property;
* equipment;
* vehicle;
* inventory;
* digital asset;
* document;
* intellectual property.

Assets may belong to:

* Person;
* Business;
* Family;
* Community.

Ownership and possession must remain distinct.

---

# 23. Money

Money is represented through financial records.

Core primitives:

```text
ACCOUNT / WALLET
TRANSACTION
CONTRIBUTION
OBLIGATION
INVOICE
PAYMENT
EXPENSE
SETTLEMENT
```

Circle does not automatically become a financial institution.

Financial records should explicitly indicate whether Circle:

* recorded the transaction;
* initiated it;
* confirmed it through a provider;
* actually holds the funds.

---

# 24. Vault

Vault is the contextual financial surface over Money objects.

It belongs to a Circle when financial capabilities are enabled.

```text
Circle
 ↓
Vault
 ↓
Financial Objects
```

Vault can exist without Circle custody.

---

# 25. Place

Place gives context to:

* Events;
* Story;
* Activities;
* Businesses;
* Assets;
* Relationships.

Place may represent:

* exact location;
* neighborhood;
* city;
* region;
* country;
* named place.

Location visibility must remain permission-controlled.

---

# 26. Media

Media represents:

* image;
* video;
* audio;
* document.

Media should be attachable to multiple contexts without unnecessary duplication.

---

# 27. Evidence

Evidence supports a claim, event, transaction, identity, or achievement.

Examples:

* receipt;
* signed document;
* photograph;
* certificate;
* transaction confirmation;
* project deliverable.

Evidence is especially important for Business and Community trust.

---

# 28. Capability

A Capability is a reusable functional module.

Examples:

* Cargo;
* Inventory;
* Booking;
* Orders;
* Voting;
* Membership;
* Property;
* Logistics.

Capabilities attach to archetypes and Circles.

They should not redefine the underlying domain.

---

# 29. Permission

Permission determines what a Person/Face can:

* see;
* create;
* edit;
* approve;
* delete;
* export;
* administer.

Permissions operate within context.

---

# 30. Interconnection Graph

At the conceptual level:

```text
                    PERSON
                       │
                    FACE
                       │
                  MEMBERSHIP
                       │
                    CIRCLE
                 /     │      \
              STORY    NOW    NEXT
                │       │       │
             OBJECTS ACTIVITIES POSSIBILITIES
                │       │       │
              PEOPLE  MONEY   COMMITMENTS
                │       │       │
              PLACES  EVIDENCE PROJECTS
```

Everything important should be linkable without becoming tightly coupled.

---

# 31. Temporal Model

Every meaningful object may have:

* created_at;
* occurred_at;
* updated_at;
* valid_from;
* valid_until;
* archived_at.

This allows Circle to distinguish:

> when something was entered

from:

> when something actually happened.

That distinction is essential for trustworthy Story.

---

# 32. State vs History

Circle must maintain two complementary concepts:

### Current State

What is true now.

### Historical Record

What was true before.

Example:

```text
Current role:
Treasurer

History:
Member
→ Organizer
→ Treasurer
```

Changing current state must not erase meaningful history.

---

# 33. Soft Deletion

Deletion should distinguish:

* hide;
* archive;
* revoke;
* delete;
* anonymize;
* legally retain.

A financial record may have different deletion requirements from a personal note.

---

# 34. Ownership

Ownership should support:

* individual ownership;
* joint ownership;
* Circle ownership;
* delegated control;
* custodial control.

Ownership is not the same as:

* access;
* possession;
* responsibility;
* authorship.

---

# 35. Consent

Consent should be modeled explicitly where needed.

Examples:

* joining Circle;
* publishing Story;
* revealing identity;
* sharing location;
* linking Faces;
* exposing financial information;
* preserving media.

---

# 36. Auditability

Important actions should leave an audit trail.

Especially:

* financial actions;
* permission changes;
* governance decisions;
* identity changes;
* membership changes;
* deletions;
* exports;
* administrative actions.

---

# 37. The Domain Model Test

A new feature should first ask:

> **Which existing domain objects does this use?**

If the answer requires creating a completely new parallel universe, the feature probably needs architectural review.

The goal is:

> **few primitives, many compositions.**

---

# 38. Core Object Set

The first implementation should prioritize:

```text
Person
Account
Face
Circle
Membership
Role
Relationship
Activity
Story
Now
Next
Possibility
Commitment
Event
Project
Asset
Money
Vault
Place
Media
Evidence
Capability
Permission
Consent
Audit Event
```

Everything else should be justified against these primitives.
