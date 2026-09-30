# CIRCLE

## Information Architecture

### Version 1.0 — Navigation & Page Structure

---

# 1. Architecture Principles

Circle's information architecture follows five rules:

1. **Keep top-level navigation small.**
2. **Circles are the primary social and organizational containers.**
3. **Me is a Living Portrait, not a static profile.**
4. **Story / Now / Next are temporal lenses, not arbitrary feature buckets.**
5. **Capabilities appear contextually instead of forcing every user to learn every feature.**

---

# 2. Primary Navigation

```text
CIRCLE
│
├── TODAY
├── CIRCLES
├── ME
├── CREATE (+)
└── GLOBAL
    ├── SEARCH
    ├── NOTIFICATIONS
    ├── SETTINGS
    └── ACCOUNT / PRIVACY
```

The earlier four-tab model of Today · Chronicle · Me · Notifications is replaced by this simpler structure.

**Chronicle remains an internal architectural concept.**

The user-facing concept is **Story**.

---

# 3. Today

### Purpose

> **What needs me now?**

Today is the default landing surface.

It is not a social feed.

### Structure

```text
TODAY
│
├── Important / urgent
├── Commitments
├── Circle activity requiring attention
├── Payments / Vault confirmations
├── Votes / approvals
├── Work reviews
├── Upcoming events
├── Setup / open threads
└── Quiet contextual possibilities
```

The content is generated from the person's actual relationships, responsibilities, Circles, and activity.

---

# 4. Circles

### Purpose

> **Who and what am I connected to?**

```text
CIRCLES
│
├── My Circles
│   ├── Family
│   ├── Fellowship
│   ├── Business
│   └── Community
│
├── Invitations
├── Linked Circles
├── Branches
└── Create Circle
```

The Circles page is not a generic people directory.

It shows Circles in which the person actually participates or has an intentional relationship.

---

# 5. Create Circle

The user should not need to understand the word “archetype” during onboarding.

Instead:

> **What are you bringing people together for?**

Options:

* Family
* Friends / Fellowship
* Work / Business
* Community

The underlying archetype is established by the selection.

Advanced configuration happens later.

---

# 6. Universal Circle Structure

Every Circle has:

```text
CIRCLE
│
├── NOW
├── STORY
├── VAULT
├── NEXT
└── SETTINGS
```

The shell is universal.

The contents and capabilities are archetype-aware.

---

# 7. Circle Header

The header provides orientation rather than becoming a dashboard.

```text
[CIRCLE IDENTITY]

Name
Context / archetype
Member count
Activity indicator
Your role
Circle switcher
More / Settings
```

---

# 8. Circle — Now

### Purpose

> **What's happening here now?**

Now contains active Circle life:

* conversation;
* voice;
* media;
* meetings;
* notes;
* status;
* active work;
* commitments;
* relevant tasks;
* current activity.

The earlier **Flow** concept belongs here.

### Permanence rule

Now is not permanent by default.

Meaningful material can be deliberately promoted into Story.

---

# 9. Circle — Story

### Purpose

> **What has this Circle lived, built, decided, or preserved?**

Story is the user-facing evolution of Chronicle.

It may contain:

* milestones;
* memories;
* documents;
* assets;
* decisions;
* completed work;
* evidence;
* selected media;
* meaningful records.

### Story principles

* searchable;
* contextual;
* durable;
* consent-aware;
* deliberately preserved;
* never silently populated with everything.

---

# 10. Circle — Vault

### Purpose

> **What does this Circle hold, contribute, owe, or record financially?**

### Family

* contributions;
* shared funds;
* dependent wallets.

### Fellowship

* IOUs;
* trip funds;
* shared expenses.

### Business

* invoices;
* payments;
* expenses;
* settlements.

### Community

* contributions;
* treasury;
* financial history.

The Vault must clearly show external-provider confirmation where applicable.

---

# 11. Circle — Next

### Purpose

> **What is coming, intended, or becoming possible?**

Examples:

* upcoming events;
* plans;
* goals;
* opportunities;
* projects;
* proposals;
* future commitments;
* potential paths.

Next should not become a generic task manager.

It is contextual to the Circle.

---

# 12. Circle — Settings

```text
SETTINGS
│
├── Members
├── Roles
├── Permissions
├── Privacy
├── Circle identity
├── Capabilities
├── Linked Circles
├── Branches
├── Vault settings
├── Notifications
├── Data / Export
└── Leave / Dissolve
```

Available options depend on role and archetype.

---

# 13. Family Circle

```text
FAMILY
│
├── NOW
│   ├── Family activity
│   ├── Events
│   ├── Needs
│   └── Commitments
│
├── STORY
│   ├── Family history
│   ├── Tree
│   ├── Memories
│   ├── Milestones
│   └── Voice heritage
│
├── VAULT
│   ├── Contributions
│   ├── Shared funds
│   └── Dependent wallets
│
├── NEXT
│   ├── Plans
│   ├── Family goals
│   ├── Education
│   └── Succession
│
└── SETTINGS
```

Family emphasizes memory, care, relationships, and continuity.

No public closeness score.

---

# 14. Fellowship Circle

```text
FELLOWSHIP
│
├── NOW
│   ├── Conversation
│   ├── Activities
│   └── Events
│
├── STORY
│   ├── Milestones
│   ├── Memories
│   └── Shared experiences
│
├── VAULT
│   ├── IOUs
│   ├── Trip funds
│   └── Shared expenses
│
├── NEXT
│   ├── Trips
│   ├── Plans
│   └── Shared possibilities
│
└── SETTINGS
```

Fellowship prioritizes closeness and shared experience.

No closeness score.

---

# 15. Business Circle

```text
BUSINESS
│
├── NOW
│   ├── Work
│   ├── Clients
│   ├── Staff
│   ├── Commitments
│   └── Active operations
│
├── STORY
│   ├── Completed work
│   ├── Evidence
│   ├── Milestones
│   └── Highlights
│
├── VAULT
│   ├── Invoices
│   ├── Payments
│   ├── Expenses
│   └── Reconciliation
│
├── NEXT
│   ├── Projects
│   ├── Opportunities
│   ├── Hiring
│   └── Expansion
│
├── CAPABILITIES
│   ├── Inventory
│   ├── Orders
│   ├── Logistics
│   ├── Cargo
│   ├── Bookings
│   └── Other enabled modules
│
└── SETTINGS
```

**Cargo is explicitly a capability, not a required Business page.**

---

# 16. Community Circle

```text
COMMUNITY
│
├── NOW
│   ├── Discussions
│   ├── Issues
│   ├── Meetings / Indaba
│   └── Active projects
│
├── STORY
│   ├── Decisions
│   ├── Bylaws
│   ├── Projects
│   └── Governance history
│
├── VAULT
│   ├── Contributions
│   ├── Treasury
│   └── Financial history
│
├── NEXT
│   ├── Proposals
│   ├── Pledges
│   ├── Projects
│   └── Upcoming decisions
│
└── SETTINGS
```

Community emphasizes collective governance and shared resources.

---

# 17. Me

Me is the person's Living Portrait.

```text
ME
│
├── STORY
├── NOW
├── NEXT
├── POSSIBILITIES
├── CIRCLES
├── PEOPLE / RELATIONSHIPS
├── FACES
└── MY DATA / CONTROL
```

### Story

Meaningful accumulated history across Circles.

### Now

Current roles, memberships, commitments, active work, and present context.

### Next

Declared intentions, goals, plans, availability, and relevant possibilities.

### Possibilities

Contextually surfaced opportunities and possible paths.

### Circles

Circles the person belongs to or intentionally links with.

### People

Relationships contextualized by Circle.

### Faces

Personal, Business, and Pseudonymous identities where applicable.

---

# 18. Me Is Not a Second Database

Me should render from existing underlying records.

Do not create a separate manually maintained portfolio when the same information already exists in:

* Story;
* work;
* relationships;
* verified activity;
* Circle membership.

This prevents duplication and contradiction.

---

# 19. Create (+)

Create is an action surface, not a place users need to browse.

Possible actions:

```text
CREATE
│
├── Circle
├── Story
├── Event
├── Work
├── Commitment
├── Transaction
├── Note
└── Possibility / Goal
```

Available actions change with context.

---

# 20. Search

Search is global but context-aware.

```text
SEARCH
│
├── People
├── Circles
├── Story
├── Activities
├── Work
├── Transactions
├── Documents
├── Places
└── Possibilities
```

Search results should preserve relationships.

For example:

> **Amina**

could show:

* Sister · Family
* Photographer · Business
* Member · Fellowship

subject to privacy and face rules.

---

# 21. Notifications

Notifications are a utility.

They should not become the main product destination.

They should be:

* contextual;
* actionable;
* grouped;
* suppressible;
* connected to real activity.

The user's primary attention question remains:

> **What needs me?**

That belongs in Today.

---

# 22. Direct Messages

Direct messages are contextual.

They should appear where a real relationship exists.

A conversation may be associated with:

* person;
* Circle;
* project;
* event;
* transaction;
* commitment.

Circle does not need a chat-shaped primary navigation item.

---

# 23. Relationships

Relationships are first-class objects.

A person may simultaneously be:

* family member;
* Fellowship member;
* business owner;
* community member;
* contractor;
* supplier;
* client.

Each relationship retains its own context and authority.

A role in one Circle does not automatically grant authority in another.

---

# 24. Branches and Affiliations

### Branches

```text
PARENT CIRCLE
│
├── BRANCH A
├── BRANCH B
└── BRANCH C
```

Each branch remains an independent Circle.

### Affiliations

```text
FAMILY ───── BUSINESS
COMMUNITY ── FAMILY
BUSINESS ─── BUSINESS
```

Each side shares only what it intentionally exposes.

---

# 25. Possibilities in the IA

Possibility Expander should appear contextually.

Examples:

### Today

> A possibility connected to something you recently completed.

### Me → Next

> Your emerging possibilities.

### Business → Next

> Opportunities connected to current work.

### Family → Next

> Potential plans connected to family goals.

### After Story

> “This experience could open…”

The same underlying intelligence serves different contexts.

---

# 26. Contextual Cards

Circle should expose interconnection through simple cards.

Example:

```text
WEBSITE REDESIGN

People
Amina · Daniel

Business
Studio X

Payment
150,000 FCFA

Story
Completed

Next
Client testimonial
```

The user sees one coherent object rather than six disconnected systems.

---

# 27. The Interconnection Rule

Every important object should be able to answer, where relevant:

* Who?
* What?
* When?
* Where?
* Which Circle?
* What relationship?
* What evidence?
* What happened?
* What is true now?
* What comes next?

Not every answer must be displayed.

The system should preserve the connections.

---

# 28. Account and Data

```text
ACCOUNT
│
├── Identity
├── Faces
├── Privacy
├── Devices
├── Recovery
├── Data Export
├── Data Deletion
├── Subscription
├── Security
└── Lifecycle Status
```

Account controls remain separate from Circle content.

---

# 29. First-Run Experience

The first-run experience should not begin with an empty dashboard.

Today can act as the initial home and guide the user toward:

> **Create your first Circle**

Then:

> **What are you bringing people together for?**

The system establishes the appropriate foundation.

---

# 30. High-Level Sitemap

```text
CIRCLE
│
├── TODAY
│
├── CIRCLES
│   │
│   ├── FAMILY
│   │   ├── NOW
│   │   ├── STORY
│   │   ├── VAULT
│   │   └── NEXT
│   │
│   ├── FELLOWSHIP
│   │   ├── NOW
│   │   ├── STORY
│   │   ├── VAULT
│   │   └── NEXT
│   │
│   ├── BUSINESS
│   │   ├── NOW
│   │   ├── STORY
│   │   ├── VAULT
│   │   ├── NEXT
│   │   └── CAPABILITIES
│   │
│   └── COMMUNITY
│       ├── NOW
│       ├── STORY
│       ├── VAULT
│       └── NEXT
│
├── ME
│   ├── STORY
│   ├── NOW
│   ├── NEXT
│   ├── POSSIBILITIES
│   ├── CIRCLES
│   ├── PEOPLE
│   └── FACES
│
├── CREATE
│
└── GLOBAL
    ├── SEARCH
    ├── NOTIFICATIONS
    ├── SETTINGS
    ├── PRIVACY
    └── ACCOUNT
```

---

# 31. Vocabulary Map

| Earlier / internal term | Preferred user-facing term                  |
| ----------------------- | ------------------------------------------- |
| Chronicle               | **Story**                                   |
| Flow                    | **Now**                                     |
| Njangi Vault            | **Vault**                                   |
| Past                    | **Story**                                   |
| Present                 | **Now**                                     |
| Future                  | **Next**                                    |
| Digital Twin            | **Living Portrait**                         |
| Possibility Expander    | **Possibilities / contextual intelligence** |
| Cargo                   | **Capability: Cargo / Logistics**           |

Cultural terms such as **Njangi, Palaver, and Indaba** can remain where they provide authentic meaning, but they should be explained rather than becoming mandatory vocabulary.

---

# 32. Final Architecture Principle

The architecture should feel like:

> **One connected life, viewed through different lenses.**

Not:

> A collection of separate apps inside Circle.

The desired movement is:

**Person → Relationship → Circle → Activity → Story → Now → Next → Possibility → Action**

without the user feeling that they have changed products.
