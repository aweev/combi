# CIRCLE

## Interconnection & Context Engine

### The system that makes Circle feel like one connected world

---

# 1. Purpose

Circle's defining technical advantage should not be the number of features it contains.

It should be the **quality of the connections between them**.

The Context Engine is responsible for understanding:

> who is connected to what, in which context, through which relationship, and why that connection matters.

---

# 2. The Core Graph

The conceptual graph is:

```text
PEOPLE
  ↕
RELATIONSHIPS
  ↕
CIRCLES
  ↕
ACTIVITIES
  ↕
OBJECTS
  ↕
STORY / NOW / NEXT
  ↕
POSSIBILITIES
```

Supporting dimensions:

```text
TIME
PLACE
MONEY
MEDIA
EVIDENCE
ROLE
PERMISSION
```

---

# 3. Why Interconnection Matters

Without interconnection:

> Circle becomes another collection of tabs.

With interconnection:

> Circle becomes a coherent model of life.

Example:

A payment is not merely:

> “150,000 FCFA transaction.”

It may be:

> payment for a project, between two people, inside a Business Circle, associated with a client relationship, attached to evidence, completed on a specific date, and potentially useful as future work history.

That context is the product.

---

# 4. Context Object

Every meaningful object should have a contextual envelope.

Conceptually:

```text
OBJECT
├── WHO
├── WHAT
├── WHEN
├── WHERE
├── WHY
├── CIRCLE
├── RELATIONSHIPS
├── STATUS
├── EVIDENCE
├── VISIBILITY
└── HISTORY
```

The UI may expose only the relevant subset.

---

# 5. Contextual Linking

Links should be explicit whenever possible.

Examples:

```text
Person
→ participated in
Event

Event
→ occurred at
Place

Event
→ belongs to
Circle

Event
→ produced
Story

Story
→ connects to
Person
```

---

# 6. Relationship Graph

Circle should model different relationship types.

Examples:

### Human

* parent;
* child;
* sibling;
* friend;
* partner;
* mentor;
* colleague.

### Business

* owner;
* employee;
* contractor;
* client;
* supplier;
* partner.

### Community

* member;
* steward;
* representative;
* contributor.

Relationship types should be extensible.

---

# 7. Relationship Strength

Circle should not reduce relationships to one numerical score.

However, internal systems may need concepts such as:

* recency;
* frequency;
* duration;
* mutuality;
* explicitness.

These should inform relevance rather than become public “relationship scores.”

---

# 8. Contextual Relevance

When deciding what to surface, Circle can consider:

```text
Relevance =
relationship
+
recency
+
importance
+
responsibility
+
context
+
explicit intent
```

The exact algorithm remains implementation-specific.

The principle is:

> **Context beats popularity.**

---

# 9. The Context Window

A user should see the smallest useful amount of connected information.

Example:

```text
PROJECT
Website Redesign

People
Amina · Daniel

Client
Studio X

Status
Completed

Payment
Confirmed

Story
Saved

Next
Request testimonial
```

The user does not need to open five separate modules.

---

# 10. Context Expansion

Every object should allow controlled expansion.

Example:

> Project

can expand to:

> People → Client → Payment → Evidence → Story → Next

The user discovers the graph progressively.

---

# 11. Context Compression

The reverse is equally important.

Circle should be able to compress many records into a meaningful summary.

Example:

> “The project was completed last Tuesday, payment was confirmed, and the client relationship remains active.”

The underlying records remain available.

---

# 12. Interconnection Without Chaos

Everything should not connect to everything.

A connection must have a meaningful semantic reason.

Bad:

> Every activity connected to every member.

Good:

> Person participated in Event.

Good:

> Event generated Story.

Good:

> Transaction settled Commitment.

---

# 13. Connection Types

Initial semantic relationships:

```text
BELONGS_TO
PARTICIPATED_IN
OWNS
MANAGES
WORKED_ON
CONNECTED_TO
RELATED_TO
OCCURRED_AT
LOCATED_AT
PAID_FOR
FULFILLS
CREATED
PRESERVED_AS
LEADS_TO
SUGGESTS
AFFILIATED_WITH
BRANCHED_FROM
```

---

# 14. Provenance

Every important connection should have provenance where relevant.

Example:

> Amina is a supplier.

How do we know?

* Business admin created relationship;
* transaction history;
* contract;
* verified provider record.

The system should distinguish:

### User-declared

Someone entered it.

### System-derived

The system inferred it.

### Provider-confirmed

External source confirmed it.

### Evidence-backed

An attached record supports it.

---

# 15. Context and Permissions

The graph must not become a privacy leak.

A relationship may exist internally without being visible to every user.

Therefore:

```text
GRAPH
+
PERMISSION
=
VISIBLE CONTEXT
```

The Context Engine must always filter through authorization.

---

# 16. Cross-Circle Connections

Cross-Circle links are powerful.

Example:

```text
Family
   ↓
Person
   ↓
Business
   ↓
Community Project
```

But the system must not expose Family information to Business merely because the same person participates in both.

Only explicitly shareable context crosses boundaries.

---

# 17. Circle-to-Circle Context

Circles can have relationships:

```text
Family
   ↕ affiliation
Business

Business
   ↕ partnership
Business

Community
   ↕ affiliation
Family
```

These are intentional links.

---

# 18. Contextual Search

Search should understand relationships.

Instead of only:

> “Daniel”

it can understand:

> “Daniel's projects with Amina.”

or:

> “Payments from the community project.”

or:

> “Our family events in Kumba.”

The underlying system resolves entities and relationships.

---

# 19. Contextual Notifications

A notification should explain why it matters.

Bad:

> “Payment received.”

Better:

> “Payment received for the completed website project.”

Best:

> “The final payment for the website project from Studio X has been confirmed.”

The second and third versions use context.

---

# 20. Contextual Creation

When a user creates something, Circle should inherit relevant context.

Example:

While viewing a Business Project:

> Create Commitment

The commitment automatically knows:

* Business;
* project;
* relevant people;
* context.

The user should not repeatedly fill in information Circle already knows.

---

# 21. Contextual Story

When something becomes Story, its relationships should remain intact.

Example:

```text
Story:
"Community Water Project Completed"

Connected:
Community
People
Project
Location
Funding
Documents
Photos
Decision
```

Story becomes far more valuable because it retains the surrounding context.

---

# 22. Contextual Possibility

Possibilities should emerge from graph relationships.

Example:

```text
Business
  ↓
Completed branding project
  ↓
Skill / evidence
  ↓
Related Business
  ↓
Similar need
  ↓
Possible introduction
```

The possibility is not random discovery.

It is graph-informed.

---

# 23. Contextual Intelligence

The intelligence layer can operate over:

* graph relationships;
* temporal information;
* explicit intentions;
* activity;
* Story;
* capabilities;
* permissions.

But every inference must preserve provenance.

---

# 24. Context Decay

Context changes.

A relationship can end.

A project can become irrelevant.

A goal can change.

A person can leave a Circle.

Therefore connections need lifecycle.

Possible states:

```text
ACTIVE
INACTIVE
EXPIRED
REVOKED
ARCHIVED
DISPUTED
```

---

# 25. Contradictions

Circle must expect conflicting information.

Example:

One person says:

> “Project completed.”

Another says:

> “Project still has outstanding work.”

The system should not silently select one as truth.

Instead:

> **Conflicting status**

with provenance.

---

# 26. Disputes

Some connections become disputed.

Examples:

* ownership;
* payment;
* relationship;
* completion;
* identity.

The system should preserve:

* claim;
* counterclaim;
* evidence;
* resolution status.

---

# 27. Temporal Context

Connections can change over time.

Example:

```text
2022 → Employee
2024 → Manager
2026 → Former employee
```

The system must answer:

> What was true then?

not only:

> What is true now?

---

# 28. Contextual Privacy

A user's view of the graph is a filtered projection.

Conceptually:

```text
FULL GRAPH
    ↓
IDENTITY FILTER
    ↓
RELATIONSHIP FILTER
    ↓
CIRCLE FILTER
    ↓
PERMISSION FILTER
    ↓
VISIBLE GRAPH
```

This should be enforced at the data layer.

---

# 29. Contextual UX

The UI should expose connections through:

* chips;
* cards;
* timelines;
* relationship labels;
* breadcrumbs;
* “because” explanations;
* expandable context.

Avoid giant graph visualizations as the default interface.

The graph is infrastructure.

The user experiences **clarity**.

---

# 30. Contextual Breadcrumb

Example:

```text
Business
→ Projects
→ Website Redesign
→ Payment
→ Story
```

This lets users understand where information belongs.

---

# 31. Contextual Backtracking

Users should be able to move backward through context.

From:

> Payment

to:

> Project

to:

> Client

to:

> Business

without losing their place.

---

# 32. Contextual Home

Today is essentially a dynamically assembled context surface.

It answers:

> “Given everything I am legitimately connected to, what matters now?”

This is why Today should remain small and focused.

---

# 33. Context Engine and the Living Portrait

The Living Portrait is a projection of the Context Graph around one Person.

```text
PERSON
│
├── Story
├── Now
├── Next
├── Relationships
├── Circles
├── Work
└── Possibilities
```

---

# 34. Context Engine and Circle

A Circle is a projection of the same graph around a collective.

```text
CIRCLE
│
├── Members
├── Relationships
├── Story
├── Now
├── Next
├── Work
├── Money
└── Possibilities
```

This is what allows Circle to remain one system.

---

# 35. Technical Principle

The underlying implementation may use:

* relational tables;
* event streams;
* indexes;
* graph-like relationship tables;
* search indexes;
* derived views.

Circle does not need a literal graph database to implement a graph model.

The conceptual model is more important than the storage technology.

---

# 36. The Context Engine Test

Whenever a new feature is proposed, ask:

> What existing context does it connect to?

Then:

> What new context does it create?

If the feature creates an isolated island, it should be reconsidered.

---

# 37. Final Principle

Circle's competitive advantage should become:

> **Not that it stores more things, but that it understands how the things that matter belong together.**

That is the Context Engine.
