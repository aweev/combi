# CIRCLE

## Identity & Privacy (merged canonical document)

**Status:** AGREED — supersedes `Identity Privacy & Trust.md` and `Identity Face & Personal Space.md`, whose full content is preserved below.
**Merge rationale:** three documents defined identity (this pair plus `Domain Model.md`), with duplicate Identity Verification sections and a Face-taxonomy conflict resolved by ADR-0007. One document, one definition per concept.

---

## PART I — Identity, Privacy, Permissions, Trust (canonical)

---

# 1. Purpose

Circle's greatest asset is not content.

It is **contextual trust**.

A person may participate in multiple parts of life simultaneously, but those contexts should not automatically collapse into one public identity.

The system therefore separates:

> **Person → Face → Relationship → Circle → Permission**

---

# 2. Identity Principles

### Principle 1

One human can have multiple legitimate contexts.

### Principle 2

A context should not automatically expose another context.

### Principle 3

Identity should be portable.

### Principle 4

Authority must be explicit.

### Principle 5

Privacy should follow relationships, not merely screens.

---

# 3. Person vs Account vs Face

These are three different concepts.

### Person

The underlying human.

### Account

The mechanism through which they access Circle.

### Face

The identity they choose to present in a particular context.

```text
PERSON
   │
ACCOUNT
   │
FACES
├── Personal
├── Business
└── Pseudonymous
```

---

# 4. Personal Face

The Personal Face represents the person's ordinary identity.

It may contain:

* preferred name;
* profile image;
* biography;
* Story highlights;
* Circles;
* selected relationships;
* contact methods.

Visibility is controlled.

Membership in Circle does not automatically make all Personal Face information visible.

---

# 5. Business Face

A Business Face allows someone to act as an organization or commercial identity.

Example:

```text
Person
   │
Business Face
   │
Business Circle
```

This prevents the person's personal identity from being confused with the business's operational identity.

Business Face may expose:

* business name;
* role;
* business contact;
* business Story;
* work history.

---

# 6. Pseudonymous Face

A Pseudonymous Face allows participation without exposing the ordinary identity.

Use cases may include:

* sensitive Fellowship;
* vulnerable community participation;
* whistleblowing-style contexts;
* identity-sensitive support groups.

Circle must protect the separation technically, not merely through UI promises.

---

# 7. Identity Linking

A person may intentionally link Faces.

Example:

> “This Business Face is operated by me.”

But such linking must be explicit.

A public viewer should not be able to infer hidden identity merely because two Faces share an underlying account.

---

# 8. Relationship Privacy

Privacy follows relationships.

Example:

A person's:

* sister;
* employer;
* friend;
* client

should not necessarily see the same information.

Visibility can depend on:

```text
Person
+
Relationship
+
Circle
+
Object
+
Permission
```

---

# 9. Visibility Levels

The system should support more than Public/Private.

Conceptual levels:

```text
ONLY ME
SPECIFIC PEOPLE
CIRCLE
LINKED CIRCLES
SELECTED AUDIENCE
PUBLIC
```

Additional restrictions may apply to sensitive domains.

---

# 10. Object-Level Privacy

Privacy attaches to objects where appropriate.

Example:

A Story may be:

> visible to Family only.

A Business document may be:

> visible to Business Admins.

A payment may be:

> visible to payer, recipient, and authorized financial roles.

---

# 11. Context Isolation

A person's participation in one Circle must not automatically reveal:

* membership in another Circle;
* private relationships;
* pseudonymous identity;
* private financial information;
* private Story.

The system must default toward contextual separation.

---

# 12. Membership Privacy

A Circle may control whether its membership is:

* hidden;
* visible to members;
* visible to invited people;
* publicly discoverable.

A person should also have controls where applicable.

---

# 13. Relationship Privacy

Some relationships are inherently sensitive.

Examples:

* family relationships;
* pseudonymous relationships;
* client relationships;
* employment relationships;
* support relationships.

These should not become public merely because they exist in the underlying graph.

---

# 14. Permission Model

Permissions are based on:

```text
SUBJECT
+
ROLE
+
CIRCLE
+
OBJECT
+
ACTION
```

Actions include:

* view;
* create;
* edit;
* approve;
* manage;
* export;
* delete;
* invite;
* remove;
* administer.

---

# 15. Role-Based Access

Circle should primarily use role-based permissions.

Example:

```text
Business Owner
→ full administrative authority

Business Manager
→ operational authority

Staff
→ assigned operational authority

Client
→ restricted project access
```

Roles can be combined with object-level restrictions.

---

# 16. Delegation

People may temporarily delegate authority.

Example:

> Treasurer → delegates payment approval to Assistant for two weeks.

Delegation must include:

* scope;
* start;
* expiration;
* revocation;
* audit record.

---

# 17. Separation of Duties

High-risk actions should not always be controlled by one person.

Example:

```text
Payment Created
      ↓
Payment Reviewed
      ↓
Payment Approved
```

This is especially important for:

* Community treasury;
* Business finance;
* high-value assets.

---

# 18. Consent

Consent must be explicit when the system crosses meaningful privacy boundaries.

Examples:

* adding someone to Circle;
* publishing their image;
* exposing their identity;
* linking Faces;
* sharing location;
* preserving a sensitive Story;
* revealing financial information.

Consent should be:

* understandable;
* contextual;
* recorded;
* revocable where possible.

---

# 19. Trust

Trust in Circle should come from **evidence and behavior**, not popularity.

Potential trust signals include:

* completed work;
* verified transactions;
* fulfilled commitments;
* authenticated membership;
* explicit vouches;
* evidence;
* long-term participation.

Circle should not create a universal “human worth” score.

---

# 20. Business Trust

Business trust can be contextual.

A customer may see:

* completed projects;
* verified transactions;
* business tenure;
* relevant capabilities;
* documented work.

The system should not automatically convert these into an arbitrary score.

---

# 21. Community Trust

Community trust may involve:

* membership history;
* fulfilled commitments;
* participation;
* governance activity;
* documented contributions.

Again:

> evidence first, score second.

Often, no score is necessary.

---

# 22. Family and Fellowship Trust

Circle should not turn intimacy into gamification.

Avoid:

* friendship scores;
* family rankings;
* loyalty points;
* closeness meters.

The product should preserve relationships rather than quantify them unnecessarily.

---

# 23. Identity Verification

Identity verification should be contextual.

Different situations may require different levels:

### Basic

Email / phone / device verification.

### Stronger

Government or provider-assisted verification where justified.

### Business

Business registration or other evidence where applicable.

### Transactional

Provider-confirmed financial identity where required.

Verification must never imply more certainty than the underlying evidence supports.

---

# 24. Evidence

Trust claims should link to evidence where appropriate.

Example:

> “Completed 14 projects”

should be traceable to actual records rather than self-declared reputation.

Evidence itself remains privacy-controlled.

---

# 25. Audit Trail

Circle must record high-impact changes.

Examples:

* role changes;
* permission changes;
* financial actions;
* membership removal;
* identity linking;
* deletion;
* exports;
* governance decisions.

Users with appropriate authority should be able to inspect relevant audit history.

---

# 26. Security Boundaries

The system must separate:

### Authentication

Who is accessing the system?

### Authorization

What may they do?

### Visibility

What may they see?

### Ownership

Who controls the object?

### Audit

What happened?

These are different concepts and should not be collapsed into one permission mechanism.

---

# 27. Data Minimization

Circle should collect information because it serves a defined purpose.

Not:

> “We may need this someday.”

Every sensitive field should have a reason.

---

# 28. Encryption

Sensitive information should be protected in transit and at rest.

For particularly sensitive domains, Circle should consider stronger protections including:

* application-level encryption;
* key separation;
* restricted administrative access;
* encrypted backups;
* selective decryption.

---

# 29. AI Privacy

Circle intelligence must operate within permission boundaries.

The AI layer should never reason across contexts merely because the same person belongs to them.

Example:

A Business Face's private client information should not become a Family recommendation.

---

# 30. AI Provenance

When Circle generates a possibility, it should be possible internally to understand:

```text
WHY?
WHAT DATA?
WHICH PERMISSIONS?
WHAT INFERENCE?
WHAT CONFIDENCE?
```

The user does not need to see all technical details every time.

But the system must be able to explain itself when needed.

---

# 31. No Silent Identity Inference

Circle should not silently reveal:

> “These two accounts are probably the same person.”

Such relationships should not become user-visible facts without appropriate evidence and permission.

---

# 32. Data Export

People should be able to retrieve their meaningful data.

Export should eventually cover:

* identity;
* Story;
* relationships;
* Circle participation;
* documents;
* media;
* financial records where permitted;
* account information.

Exports should preserve relationships where technically possible.

---

# 33. Data Deletion

Deletion must distinguish:

* personal deletion;
* object deletion;
* Circle departure;
* account closure;
* legal retention.

Some records may require retention for:

* financial integrity;
* disputes;
* legal obligations;
* audit requirements.

When retention is necessary, Circle should minimize and protect the retained data.

---

# 34. Leaving a Circle

Leaving should not necessarily destroy history.

Example:

A person leaves a Business.

The Business's historical records may remain.

The person's active membership ends.

Their private personal information should not automatically remain exposed.

The system must separate:

> historical record

from:

> ongoing access.

---

# 35. Removing a Person

Removal requires context.

Possible actions:

```text
SUSPEND
REMOVE ACCESS
REMOVE MEMBERSHIP
ANONYMIZE
DELETE
RETAIN FOR AUDIT
```

The correct action depends on the object and legal context.

---

# 36. Circle Dissolution

A Circle may be:

* archived;
* dissolved;
* transferred;
* merged where supported.

Important Story and financial records require explicit lifecycle rules.

---

# 37. Succession

Succession is especially important for:

* Family;
* Business;
* Community.

A Circle may define:

> who assumes administrative responsibility if the current administrator becomes unavailable.

Succession must be explicit, auditable, and securely recoverable.

---

# 38. Trust Without Surveillance

Circle should never confuse:

> accountability

with:

> constant observation.

The product should preserve meaningful evidence while minimizing unnecessary monitoring.

---

# 39. Safety

Circle should provide mechanisms for:

* reporting;
* blocking;
* moderation;
* abuse prevention;
* fraud detection;
* account recovery;
* dispute handling.

Safety mechanisms must themselves respect privacy and due process.

---

# 40. Trust Architecture

The full trust model is:

```text
IDENTITY
   ↓
AUTHENTICATION
   ↓
RELATIONSHIP
   ↓
ROLE
   ↓
PERMISSION
   ↓
ACTION
   ↓
EVIDENCE
   ↓
AUDIT
   ↓
TRUST
```

Trust is therefore not a badge.

It is the accumulated result of:

> **who someone is, what they are allowed to do, what they actually did, and what evidence remains.**

---

# 41. Final Principle

Circle should make people feel:

> **“I know who can see this.”**

> **“I know why they can see it.”**

> **“I know who is responsible.”**

> **“I can trust the history because it is connected to reality.”**

That is the foundation on which the rest of Circle can safely grow.

---

## PART II — Face & Personal Space (formerly `Identity Face & Personal Space.md`)

---

# 1. Purpose

Circle begins with a person.

Before a person creates a Family, Business, Fellowship, Community, or any other Circle, there must be a simple answer to:

> **Who are you here?**

The answer is the **Face**.

The Face is not merely a profile page.

It is the person's living doorway into Circle.

---

# 2. The Face

The Face represents the person within Circle.

It can contain:

* identity;
* introduction;
* relationships;
* Story;
* Now;
* Next;
* Possibilities;
* Circles;
* selected achievements;
* selected work;
* selected interests;
* public or shared information.

---

# 3. Face ≠ Profile

A conventional profile is primarily:

> name + photo + biography.

A Circle Face is:

> **identity + context + continuity + possibility.**

The Face should therefore evolve as the person's life evolves.

---

# 4. The Living Face

The Face is continuously shaped by:

```text
STORY
NOW
NEXT
RELATIONSHIPS
CIRCLES
POSSIBILITIES
```

But the user remains in control of what others see.

---

# 5. Past / Present / Future

The Face should support the simple temporal structure:

### Story

Where I've been.

### Now

Where I am.

### Next

Where I'm going.

### Possibilities

What might be possible.

This replaces the more abstract terminology of "Past / Present / Future" in the interface while preserving the underlying concept.

---

# 6. Public Face *(visibility, not a Face type — ADR-0007)*

> **Taxonomy note.** The Face *types* are exactly three — `PERSONAL`, `BUSINESS`,
> `PSEUDONYMOUS` (`Domain Model.md` §5, Part I §4-6).
> "Public" and "Private" are **visibility levels** (Part I §9 of this
> document), not kinds of Face. This section describes a Personal Face set to
> discoverable; it is not a fourth identity type.

A person may choose to make a portion of their Face discoverable.

A public Face might contain:

* name;
* photo;
* introduction;
* selected Story;
* current work;
* selected interests;
* public Circles;
* ways to connect.

It should never expose private context merely because the Face is public.

---

# 7. Private Space *(a mode of a Personal Face — ADR-0007)*

Private Space is a **scope rule over Story/Next objects owned by a Person**, not
a separate primitive and not a Face type: inventing an object for it would violate
the Expansion Rule (`Core Objects & Capability Foundation.md` §41).

Every person has a private space.

This is where they can maintain:

* private Story;
* private Next;
* personal notes;
* private Possibilities;
* saved information;
* personal planning.

The private space belongs to the person rather than to any Circle.

---

# 8. Shared Context

Some information belongs to a relationship or Circle rather than exclusively to one person.

Example:

> A wedding photograph.

It may be:

* personally meaningful to Amina;
* part of the Family Story;
* shared with selected friends;
* publicly visible as an event.

The system should allow multiple legitimate contexts without duplicating the underlying object unnecessarily.

---

# 9. Identity Layers

Circle should distinguish:

```text
IDENTITY
    ↓
FACE
    ↓
CIRCLE MEMBERSHIP
    ↓
ROLE
    ↓
PERMISSIONS
```

Being someone's friend does not automatically make them a member of a Business.

Being a Business member does not automatically expose Family information.

---

# 10. Names and Handles

A person may have:

* legal/verified identity where required;
* display name;
* username;
* public handle;
* phone/email identifiers.

These must not be treated as the same thing.

---

# 11. Identity Verification

Some Circle capabilities may require stronger verification.

Examples:

* financial actions;
* Business administration;
* ownership claims;
* governance;
* account recovery.

Verification should be capability-specific rather than unnecessarily imposed on everyone.

---

# 12. Multiple Contexts

A person can occupy different roles in different Circles.

Example:

```text
Amina
│
├── Family → daughter
├── Business → founder
├── Fellowship → organizer
└── Community → contributor
```

Circle should not force one global role onto the person.

---

# 13. Role Context

Roles belong to relationships.

Therefore:

> **A role is not an identity.**

Someone is not permanently a "manager."

They may be:

> manager of Business A

during a particular period.

---

# 14. Personal Timeline

The Face can provide a chronological view of meaningful Story.

Example:

```text
2018
Education milestone

2021
Started business

2023
Moved location

2025
Started community project

2026
Current work
```

The user decides what becomes part of the visible timeline.

---

# 15. Personal Story Selection

Not everything in a person's Story should become part of their public Face.

Users can select:

* featured Story;
* private Story;
* Circle-only Story;
* archived Story.

---

# 16. Personal Now

Now should answer:

> **What matters to me right now?**

It may include:

* active commitments;
* upcoming events;
* projects;
* relationships needing attention;
* recent meaningful activity.

It should not become a generic social feed.

---

# 17. Personal Next

Next represents intentional direction.

Examples:

* finish a project;
* learn a skill;
* visit someone;
* launch a business;
* organize a family event.

The system should preserve the distinction between:

> **what I said I want**

and:

> **what Circle thinks might be possible.**

---

# 18. Personal Possibilities

Possibility Expander is naturally rooted in the Face.

It can surface:

> "Given who you are, what you've done, who you know, and what you're exploring, what else could be possible?"

But the system must never imply that an inferred possibility is an obligation.

---

# 19. Possibility Boundaries

A possibility can be:

* explored;
* saved;
* shared;
* dismissed;
* converted to Next.

It cannot silently become a commitment.

---

# 20. Connection Requests

Connection should be contextual.

Instead of:

> "John wants to follow you."

Circle may eventually support:

> "John would like to connect with you."

The distinction is important.

Circle is fundamentally about relationships, not follower counts.

---

# 21. No Follower Economy

The product should avoid making:

* follower count;
* likes;
* popularity;
* virality

the primary measures of social value.

A person's meaningful relationships should matter more than audience size.

---

# 22. Trust

Trust can be contextual.

Examples:

* verified identity;
* known relationship;
* Circle membership;
* shared history;
* verified business;
* evidence-backed contribution.

Circle should avoid reducing trust to a simplistic universal score.

---

# 23. Privacy

Every object associated with the Face should have visibility controls.

Possible levels:

```text
ONLY ME
SELECTED PEOPLE
RELATIONSHIP
CIRCLE
CONNECTED CIRCLES
PUBLIC
```

Not every object needs every level.

---

# 24. Consent

When information crosses from one context to another, the system should respect:

* ownership;
* consent;
* visibility;
* role;
* purpose.

The fact that someone can see information in one Circle does not automatically mean they can see it elsewhere.

---

# 25. Personal Data Boundary

The Face is the user's representation.

Circle must distinguish:

> information about me

from:

> information I have access to.

Access to a document does not make the document the user's property.

---

# 26. Face as Landing Page

The Face can serve as an individual's Circle landing page.

A visitor should quickly understand:

```text
WHO
↓
WHAT MATTERS
↓
WHAT THEY ARE DOING
↓
WHAT THEY HAVE DONE
↓
WHERE THEY ARE GOING
↓
HOW TO CONNECT
```

---

# 27. Face Evolution

The Face should become richer naturally through use.

A person should not have to manually maintain dozens of profile fields.

Circle can derive candidates from activity and ask:

> "Would you like to add this to your Face?"

---

# 28. User Control

Derived information should never automatically become public identity.

The system may suggest:

> "You completed three projects related to photography."

The user decides whether that becomes:

> a capability;

> a Story;

> a featured achievement;

> or nothing.

---

# 29. Face and Circles

A person's Face is the intersection point of their Circles.

```text
              FAMILY
                 │
                 │
BUSINESS ─── PERSON ─── FELLOWSHIP
                 │
                 │
             COMMUNITY
```

Each Circle contributes context without merging all contexts together.

---

# 30. The Face as a Doorway

The fundamental UX principle is:

> **A Face should help someone understand a person without requiring them to understand Circle first.**

It must remain human-readable.

---

# 31. Final Principle

Circle should not ask:

> "What kind of profile do you want?"

It should ask:

> **"What part of yourself do you want this Circle to see?"**

The Face is therefore not a profile.

It is a **controlled window into a person's living context**.
