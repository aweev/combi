# CIRCLE

## Legal, Trust & Safety

### The framework for responsible participation, ownership, privacy, and protection

---

# 1. Purpose

Circle is designed around real people, relationships, organizations, memories, communication, documents, transactions, and possibilities.

That creates responsibility.

The platform must establish clear rules for:

* identity;
* ownership;
* privacy;
* consent;
* communication;
* content;
* intellectual property;
* moderation;
* account security;
* data retention;
* deletion;
* disputes;
* abuse;
* organizational responsibility.

---

# 2. Core Principle

> **Trust should be built into Circle's architecture, not added after the product is finished.**

---

# 3. Legal Foundation

Circle should maintain clear legal documents covering:

1. Terms of Service
2. Privacy Policy
3. Community Guidelines / Code of Conduct
4. Copyright & Takedown Policy
5. Data Retention & Deletion Policy
6. Cookie Policy / Consent
7. Business / Organization Terms where required
8. Payment and Subscription Terms where required

---

# 4. Terms of Service

The Terms should establish:

* who may use Circle;
* account responsibilities;
* acceptable use;
* intellectual-property responsibilities;
* payment obligations;
* account suspension;
* account termination;
* disputes;
* limitations of liability;
* service changes.

The language should remain understandable.

---

# 5. Privacy

Privacy must operate at the object and relationship level.

A user may share:

* one Story publicly;
* another Story with Family;
* another with a Business;
* another with selected people;
* another with nobody.

Privacy should therefore not be reduced to:

> public / private.

---

# 6. Privacy Layers

Circle can support:

```text
ONLY ME
SELECTED PEOPLE
RELATIONSHIP
CIRCLE
CONNECTED CIRCLES
PUBLIC
```

Availability should depend on the object and context.

---

# 7. Consent

Consent should be explicit when necessary.

Examples:

* sharing another person's information;
* publishing media containing others;
* joining a Circle;
* revealing contact information;
* sharing personal records;
* using information for particular purposes.

---

# 8. Contextual Consent

Consent given in one context should not automatically become universal consent.

Example:

> A person gives a Business access to their phone number.

That does not automatically mean:

> every Business member can see it.

---

# 9. Identity

Circle should distinguish:

* identity;
* display name;
* verification;
* role;
* ownership;
* permissions.

These should never be treated as one concept.

---

# 10. Verification

Verification should be proportional to risk.

Potential verification levels:

```text
UNVERIFIED
BASIC
VERIFIED
ORGANIZATION VERIFIED
HIGH-TRUST / SPECIAL VERIFICATION
```

Verification should not become a status competition.

---

# 11. Account Security

Circle should support:

* secure authentication;
* multi-factor authentication;
* session management;
* recovery;
* suspicious-login detection;
* device management;
* account security notifications.

---

# 12. Ownership

Circle must distinguish:

> creator

> owner

> administrator

> manager

> participant

> viewer

> subject

These relationships can be different.

---

# 13. Content Ownership

Users generally retain appropriate rights in content they create, subject to the legal terms governing the service.

Circle should clearly explain what rights users grant the platform to operate the service.

---

# 14. Copyright

Users must not be encouraged to upload or distribute material they do not have the right to use.

Circle should provide a clear copyright complaint process.

---

# 15. Takedown Process

Circle should maintain a dedicated copyright/takedown process.

This is distinct from:

> Report Content.

A takedown request can concern content that was legitimately published initially but is later disputed.

---

# 16. Reporting

Users should be able to report:

* harassment;
* impersonation;
* fraud;
* spam;
* illegal content;
* abuse;
* privacy violations;
* copyright issues;
* dangerous activity.

---

# 17. Report ≠ Takedown

The system should distinguish:

```text
REPORT
→ moderation review

COPYRIGHT CLAIM
→ rights/takedown process

PRIVACY REQUEST
→ privacy process

SECURITY REPORT
→ security process
```

---

# 18. Moderation

Moderation should operate across:

* content;
* accounts;
* relationships;
* communication;
* Circles.

---

# 19. Moderation Principles

Moderation should be:

* understandable;
* proportionate;
* documented;
* appealable;
* consistent.

---

# 20. Automated Moderation

Automation can help detect:

* spam;
* scams;
* abusive behavior;
* suspicious activity;
* prohibited content.

Automated systems should not automatically make high-impact decisions where meaningful human review is necessary.

---

# 21. Appeals

Users should have a way to appeal significant moderation actions.

Appeals should explain:

* what happened;
* what rule was involved;
* what action was taken;
* what can be appealed;
* how to appeal.

---

# 22. Blocking

Users should be able to block people.

Blocking should affect relevant:

* communication;
* visibility;
* interaction;
* invitations.

---

# 23. Relationship Safety

Circle should allow users to end or restrict relationships without requiring them to delete unrelated information.

Example:

> Leave Fellowship

should not necessarily delete:

> personal Story created while participating in it.

---

# 24. Leaving a Circle

When someone leaves a Circle:

* access should be revoked;
* ownership should be handled explicitly;
* their own content should follow applicable rules;
* shared content should retain appropriate provenance;
* historical participation may remain where necessary.

---

# 25. Circle Administration

Administrators should have clearly defined authority.

Admin access should not imply unrestricted access to every private object.

---

# 26. Organizational Accountability

Businesses, Families, Fellowships, and Communities may have multiple administrators.

Circle should support:

* multiple administrators;
* role separation;
* transfer of ownership;
* succession;
* emergency recovery.

---

# 27. Auditability

Important actions should be auditable.

Examples:

* permission changes;
* ownership changes;
* financial actions;
* moderation actions;
* administrative changes.

---

# 28. Data Retention

Circle should define how long different classes of information are retained.

Retention may vary according to:

* legal obligations;
* account state;
* Circle requirements;
* financial records;
* security logs;
* user requests.

---

# 29. Account Deletion

Users should be able to request account deletion.

The system should explain:

* what is deleted;
* what is anonymized;
* what must be retained;
* what happens to shared content;
* what happens to Circle ownership.

---

# 30. Dormant Accounts

Account inactivity should not automatically mean immediate deletion.

However, the platform may establish lifecycle rules for inactive accounts.

These must be clearly communicated.

---

# 31. Subscription Expiry

Where an account is subscription-dependent, Circle should distinguish:

> subscription inactive

from:

> account deleted.

The deletion lifecycle should be explicit.

---

# 32. Three-Month Inactivity Rule (revised — ADR-0005)

This section previously described a dormancy → **account removal** pathway. It is
withdrawn: removal on dormancy conflicted with the Pricing specification's §19
(Limited Mode), the Monetization specification's §22, the Lifecycle
specification's §12 (which defines "inactive subscription" as a *paid* state and
explicitly excludes "user did not open the app"), and the product North Star of
continuity.

Current legal position:

```text
SUBSCRIPTION ENDS (paid plans only)
       ↓
GRACE              (warned, time-boxed)
       ↓
RESTRICTED         (Limited Mode — data intact)
       ↓
INACTIVE → CLOSED  (commercial relationship ends — DATA INTACT)
```

**Deletion is never a consequence of non-payment or dormancy.** It arises only
from (a) an explicit user request, or (b) a legal or regulatory obligation. In
both cases the user receives advance warning and a genuine opportunity to export.

Lawful basis note: where personal data is retained after a commercial
relationship closes, retention is justified by the legitimate interest of
preserving **other members'** shared history (`Lifecycle & Account Status.md` §40,
the shared-data problem) — and personal fields not required by that interest are
anonymised rather than deleted (see
`Lifecycle & Account Status.md` §36 on anonymisation, minimisation and justified retention).

---

# 33. Number / Capacity Reclamation (revised — ADR-0003, ADR-0005)

**Identifiers and usernames are not reclaimed from dormant or lapsed accounts.**
Reusing a human identifier that once carried someone's family or financial history
creates permanent mis-association and privacy exposure. If the namespace becomes
scarce it is expanded (ADR-0003, `Lifecycle & Account Status.md` §15-16).

What *may* be released on closure is genuinely fungible capacity: unused
metered allowance (CCU) and provisioned technical resources.

The distinctions this section demanded remain mandatory and are now explicit in
the entity model (PRD §79, ADR-0003):

* **user identity** — the Person; never a billing unit, never recycled;
* **public username** — a display handle; reclamation only via explicit request;
* **billing seat** — a CCU dimension; released when the plan lapses;
* **platform capacity** — technical resource; scales down on dormancy.

These must not be conflated.

---

# 34. Rejoining (revised — ADR-0005)

A returning person reactivates against **their own account and their own
history**, after identity verification (`Lifecycle & Account Status.md` §37). If
their paid plan had closed, a new subscription starts on the same account.

There is no "previous position" to lose, because identifiers are not reclaimed
(§33) and no waitlist exists (`Monetization & Capacity Strategy.md` §26, deleted).

The only genuine rejoining case is a **requested deletion that completed** — at
which point the person is a new user, and any shared history that survived their
deletion (`Lifecycle & Account Status.md` §40) is handled under the normal
data-subject rules rather than by re-linking an old identity.

---

# 35. Children's Safety

Where minors may use Circle, the product must comply with applicable age and child-safety requirements.

Special handling may be needed for:

* consent;
* messaging;
* discovery;
* public profiles;
* location;
* media.

---

# 36. Sensitive Information

Circle should minimize unnecessary collection of sensitive information.

The system should ask:

> **Do we actually need this?**

before collecting it.

---

# 37. AI Safety

AI systems should:

* explain meaningful recommendations;
* respect permissions;
* avoid exposing private information;
* avoid inventing relationships;
* avoid silently modifying important records;
* preserve human control.

---

# 38. Trust Without a Universal Score

Circle should not reduce a person to:

> "Trust = 82."

Instead, trust should emerge from contextual evidence:

* verified identity;
* relationship;
* history;
* membership;
* role;
* documented activity.

---

# 39. Transparency

Users should understand:

* why they see something;
* why they received a notification;
* why something was suggested;
* who can access something;
* what happens when they delete it.

---

# 40. Safety Principle

> **Circle should protect relationships without becoming the judge of relationships.**

---

# 41. Final Principle

Trust is not one feature.

It is the result of:

```text
IDENTITY
+
CONSENT
+
PRIVACY
+
SECURITY
+
TRANSPARENCY
+
ACCOUNTABILITY
+
USER CONTROL
```

That combination should be present throughout the entire product.
