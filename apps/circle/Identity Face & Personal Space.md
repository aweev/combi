# CIRCLE

## Identity, Face & Personal Space

### The human identity layer of Circle

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
> `PSEUDONYMOUS` (`Domain Model.md` §5, `Identity Privacy & Trust.md` §4-6).
> "Public" and "Private" are **visibility levels** (`Identity Privacy & Trust.md`
> §9), not kinds of Face. This section describes a Personal Face set to
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
