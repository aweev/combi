# CIRCLE

## Data Architecture & Technical Foundations

### The technical foundation for an interconnected platform

---

# 1. Purpose

Circle's technical architecture must support:

* people;
* Circles;
* relationships;
* content;
* capabilities;
* permissions;
* time;
* history;
* discovery;
* multilingual content;
* monetization;
* future scale.

The architecture should remain understandable even as the product grows.

---

# 2. Technical Principle

> **Build a strong relational foundation first. Add specialized systems only when the foundation genuinely requires them.**

---

# 3. Core Domain Model

The technical model should center around:

```text
PERSON
CIRCLE
MEMBERSHIP
RELATIONSHIP
OBJECT
ACTIVITY
STORY
EVENT
COMMITMENT
PROJECT
DOCUMENT
MEDIA
TRANSACTION
CAPABILITY
PERMISSION
```

---

# 4. Person

A Person represents an identity.

Core attributes may include:

* internal ID;
* display name;
* identity information;
* preferences;
* locale;
* account state;
* timestamps.

---

# 5. Circle

A Circle represents a collective context.

Core attributes:

* ID;
* name;
* archetype;
* owner;
* lifecycle state;
* visibility;
* configuration;
* timestamps.

---

# 6. Membership

Membership connects:

```text
PERSON ↔ CIRCLE
```

It should contain:

* role;
* status;
* permissions;
* invitation state;
* joined date;
* departed date.

---

# 7. Relationship

Relationships connect entities.

The system should support:

```text
PERSON ↔ PERSON
PERSON ↔ CIRCLE
CIRCLE ↔ CIRCLE
```

Relationships may have:

* type;
* direction;
* status;
* provenance;
* visibility;
* lifecycle.

---

# 8. Role

Roles should be contextual.

A Person can have:

> Admin

in one Circle,

and:

> Member

in another.

Roles should not be global identity properties.

---

# 9. Permission Architecture

Permissions should be evaluated through:

```text
USER
↓
MEMBERSHIP
↓
ROLE
↓
CAPABILITY
↓
OBJECT
↓
ACTION
```

---

# 10. Row-Level Security

Where supported by the database architecture, access control should be enforced at the data layer as well as the application layer.

The application should never be the only security boundary.

---

# 11. Policy Design

Policies should be:

* explicit;
* composable;
* testable;
* documented.

Avoid circular authorization dependencies.

---

# 12. Authorization Helpers

Authorization functions should remain small and predictable.

For example:

```text
is_member()
is_admin()
can_view()
can_edit()
can_manage()
```

Complex permission logic should not be hidden inside opaque functions.

---

# 13. Migration Discipline

Schema changes should be versioned.

Every structural change should have:

* migration;
* rollback strategy where practical;
* test;
* documentation.

---

# 14. Dependency-Aware Migrations

Functions referenced by policies should not be dropped or replaced blindly.

Migration order should account for:

```text
FUNCTION
↓
POLICY
↓
TABLE
```

or the reverse dependency order when replacing them.

---

# 15. The `is_staff()` Lesson

The architecture should avoid tightly coupling dozens of policies to one implementation-specific helper.

If a helper changes, dependent authorization policies should be discoverable and safely migratable.

---

# 16. Schema Organization

The database should be organized by domain where useful.

Potential areas:

```text
identity
circles
relationships
content
communication
commerce
security
billing
localization
discovery
```

The exact physical schema can vary according to implementation.

---

# 17. IDs

Internal identifiers should be stable and non-semantic.

Do not encode business meaning unnecessarily into IDs.

---

# 18. Timestamps

Important entities should record:

* created_at;
* updated_at;
* relevant lifecycle timestamps.

Time-sensitive entities may require additional timestamps.

---

# 19. Soft Deletion

Where appropriate, deletion can initially mean:

> no longer active / visible.

This allows recovery and audit.

Permanent deletion should be a deliberate process.

---

# 20. Versioning

Important objects may require version history.

Examples:

* Documents;
* Policies;
* Agreements;
* Stories;
* configuration;
* permissions.

---

# 21. Audit Events

Sensitive actions should generate immutable or appropriately protected audit records.

Examples:

* ownership changes;
* permissions;
* payments;
* account security;
* moderation.

---

# 22. Event Architecture

Circle can use an internal event model.

Example:

```text
ProjectCompleted
       ↓
Activity
       ↓
Notification
       ↓
StorySuggestion
```

This allows different capabilities to react without tightly coupling every module.

---

# 23. Domain Events

Domain events should describe:

> something that happened.

They should not become arbitrary commands.

---

# 24. Asynchronous Processing

Tasks such as:

* translation;
* media processing;
* notifications;
* search indexing;
* AI analysis;

should generally run asynchronously when appropriate.

The user should not wait unnecessarily for background work.

---

# 25. Search Index

Search should have a dedicated indexing layer when scale requires it.

The database remains the source of truth.

---

# 26. Media

Media should be stored separately from relational records where appropriate.

The database stores metadata and relationships.

Object storage stores the actual files.

---

# 27. Documents

Documents should support:

* metadata;
* permissions;
* versioning;
* relationships;
* retention.

---

# 28. Secrets

Sensitive credentials and secrets should never be stored as ordinary user-accessible data.

---

# 29. Encryption

Sensitive information should use appropriate encryption:

* in transit;
* at rest;
* additional application-level encryption where necessary.

---

# 30. Backups

Circle should maintain:

* automated backups;
* tested recovery;
* disaster recovery procedures;
* appropriate retention.

A backup that has never been restored is not a proven backup.

---

# 31. Observability

The platform should monitor:

* errors;
* latency;
* availability;
* background jobs;
* database health;
* security events.

---

# 32. Reliability

Important workflows should be designed for retries.

Examples:

> payment processing;

> notification delivery;

> media processing.

Duplicate execution must be handled safely.

---

# 33. Idempotency

Operations that can be retried should use idempotency where appropriate.

This prevents:

> one action becoming two transactions.

---

# 34. API Architecture

The API should expose domain concepts rather than database implementation details.

Example:

Prefer:

> createCircle()

over exposing arbitrary table mutations.

---

# 35. Internal Services

Circle should avoid prematurely splitting into many microservices.

A modular monolith can be preferable in early development.

The architecture should separate domains logically before separating infrastructure physically.

---

# 36. Scalability

The system should be designed so individual components can scale independently when necessary.

Potential scaling boundaries:

* search;
* media;
* notifications;
* AI;
* analytics;
* payments.

---

# 37. Caching

Caching should improve performance without becoming the authoritative data source.

---

# 38. Offline / Poor Connectivity

Given environments with unreliable connectivity, Circle should eventually consider:

* resilient uploads;
* retry;
* offline drafts;
* local caching;
* synchronization.

This should be especially important for mobile.

---

# 39. Sync

Synchronization must handle:

* duplicate actions;
* conflicting edits;
* stale state;
* interrupted uploads.

---

# 40. Localization Data

Localization should be data-driven.

Translations should not require structural database duplication of entire objects.

---

# 41. Configuration

Archetypes and capabilities should be configuration-driven where practical.

Example:

```text
Business
→ recommended capabilities

Family
→ recommended capabilities

Fellowship
→ recommended capabilities
```

---

# 42. Feature Flags

Feature flags can control gradual rollout.

They should not become permanent hidden architecture.

---

# 43. Billing Architecture

Billing should be independent from identity.

The system should distinguish:

```text
PERSON
CIRCLE
SUBSCRIPTION
PLAN
BILLING ACCOUNT
ENTITLEMENT
```

---

# 44. Archetype Pricing

Each archetype may have different pricing or included capabilities.

However, pricing logic should remain configurable.

Do not hardcode:

> Business = X

into the entire application.

---

# 45. Usage / Capacity

If Circle uses progressive pricing based on scale, usage should be tracked independently.

For example:

```text
CAPACITY
→ current allocation

USAGE
→ current consumption

PLAN
→ pricing rules

ENTITLEMENT
→ what is available
```

---

# 46. Progressive Pricing Foundation

Per ADR-0003 / ADR-0004 the engine **composes** a price; it does not scale one
with platform size.

```text
PRICE = plan_base
      + sum(metered CCU dimension over allowance)
      - billing_rules(sponsorship, nonprofit, education)
      bounded_by(rate_cap = 1 band per billing year)
      bounded_by(absolute_cap, ORGANIZATION only)
```

Tier escalation, where it applies at all, is **linear and capped** — never
exponential. Numbers and currency live in
[Monetization & Economics.md](Monetization%20&%20Economics.md), not here.

---

# 47. Subscription Lifecycle

States are canonical in `Lifecycle & Account Status.md` §10 (ADR-0001) and must
not be redefined here.

```text
Subscription:  TRIAL · ACTIVE · PAST_DUE · GRACE · RESTRICTED · INACTIVE · CLOSED
Data:          ACTIVE · ARCHIVED · DELETION_PENDING · DELETED
```

Two implementation constraints follow:

1. The two enums live in **separate columns/tables** and are never derived from
   one another. `RESTRICTED` is not `DELETION_PENDING`.
2. No billing worker may write a data-lifecycle state. Deletion is triggered only
   by an explicit user request or a legal obligation (ADR-0005).

---

# 48. Subscription → Account Lifecycle

Subscription state should not automatically equal account state.

Example:

```text
subscription expires
        ↓
account remains
        ↓
grace / inactive period
        ↓
warning
        ↓
deletion eligibility
        ↓
deletion
```

---

# 49. Capacity Reclamation

If the pricing model includes scarce numbered capacity, the system should track allocation separately from account identity.

This prevents the dangerous assumption:

> deleted account = permanently deleted number.

---

# 50. Re-entry

If a user returns after deletion, the system should create or restore only what the retention rules permit.

Previously occupied capacity may no longer be available.

---

# 51. AI Architecture

AI should interact with Circle through controlled interfaces.

AI should not have unrestricted database access.

---

# 52. AI Permissions

AI actions must inherit the user's effective permissions.

If the user cannot access an object:

> AI cannot access it on their behalf.

---

# 53. AI Writes

AI-generated changes should ideally use explicit action boundaries.

Example:

```text
AI SUGGESTION
      ↓
USER CONFIRMATION
      ↓
SYSTEM ACTION
```

Automatic writes may be allowed only for low-risk operations with clear controls.

---

# 54. AI Provenance

AI-generated content should retain provenance where appropriate.

The system should distinguish:

* human-created;
* AI-assisted;
* AI-generated;
* imported.

---

# 55. Import Architecture

Circle should support future import from:

* contacts;
* calendars;
* files;
* social exports;
* business systems.

Imports should preserve provenance.

---

# 56. Export

Users should be able to export appropriate personal data.

Portability should be considered from the beginning.

---

# 57. Interoperability

Circle should avoid unnecessary proprietary lock-in.

Where useful, it should support standards and documented APIs.

---

# 58. Testing

Testing should cover:

### Unit

Individual logic.

### Integration

Domain interactions.

### Authorization

Who can access what.

### Migration

Schema changes.

### Localization

Language behavior.

### Billing

Subscription and entitlement behavior.

### Security

Abuse and attack scenarios.

---

# 59. Seed Data

Development environments should include realistic but fictional:

* People;
* Circles;
* memberships;
* Story;
* Projects;
* Events;
* capabilities.

This allows interconnected UX to be tested properly.

---

# 60. Development Principle

Do not build:

> page by page.

Build:

> domain by domain.

Then expose those domains through pages and flows.

---

# 61. Technical Golden Rule

> **The database should model reality; the interface should simplify reality.**

---

# 62. Architecture Golden Rule

> **One source of truth, many contextual views.**

A Story should not become five unrelated copies because five Circles reference it.

---

# 63. Final Technical Model

```text
                         IDENTITY
                            │
                         PEOPLE
                            │
                      RELATIONSHIPS
                            │
                         CIRCLES
                            │
          ┌─────────────────┼─────────────────┐
          ↓                 ↓                 ↓
       OBJECTS           ACTIVITY           EVENTS
          │                 │                 │
          └─────────────────┼─────────────────┘
                            ↓
                       DOMAIN EVENTS
                            │
        ┌───────────────────┼───────────────────┐
        ↓                   ↓                   ↓
   NOTIFICATIONS         SEARCH             AI
        │                   │                   │
        └───────────────────┼───────────────────┘
                            ↓
                       EXPERIENCE
                            │
                     POSSIBILITIES
                            │
                            ↓
                           NEXT
```

---

# 64. Final Principle

Circle's technical architecture should disappear beneath the experience.

The user should experience:

> **one simple, connected world.**

The system underneath should be powerful enough to support:

> people + relationships + Circles + history + action + possibility.

That is the technical foundation on which the rest of Circle can safely be built.
