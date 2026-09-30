# CIRCLE

## Archetypes & Circle Types

### One foundation, many forms of human organization

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
