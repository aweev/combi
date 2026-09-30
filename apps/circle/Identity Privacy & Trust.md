# CIRCLE

## Identity, Privacy & Trust

### The rules governing who someone is, what they can see, and why others can trust the system

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
