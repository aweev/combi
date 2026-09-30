# CIRCLE

## Lifecycle & Account Status

### What happens from creation to long-term inactivity, closure, deletion, and return

---

# 1. Purpose

Circle needs a precise lifecycle model so that:

* users know what happens to their account;
* capacity can be reclaimed;
* data is protected;
* subscriptions are understandable;
* inactive accounts do not consume scarce resources forever;
* returning users have a predictable path.

---

# 2. Lifecycle Layers

There are several independent lifecycles.

```text
ACCOUNT
SUBSCRIPTION
CIRCLE
MEMBERSHIP
OBJECT
DATA
IDENTIFIER
```

They must not be collapsed into one status.

---

# 3. Account Lifecycle

Conceptually:

```text
INVITED
  ↓
CREATED
  ↓
ACTIVE
  ↓
INACTIVE
  ↓
DORMANT
  ↓
CLOSED
  ↓
DELETED
```

Some states may be skipped.

---

# 4. Created

The user has created an account but may not yet have completed setup.

Possible status:

> **Setup**

The user can:

* verify identity;
* create Face;
* join/create Circle;
* configure privacy.

---

# 5. Active

The account is currently usable.

Active means:

* authenticated;
* within applicable subscription state;
* not suspended;
* not closed.

---

# 6. Inactive

Inactive should mean:

> the account has not had meaningful activity for a defined period.

The exact inactivity threshold should be configurable.

Inactive does not immediately mean deleted.

---

# 7. Dormant

Dormant indicates prolonged inactivity.

The system may:

* reduce background processing;
* stop expensive services;
* remove scarce allocations;
* reduce active capacity consumption.

But the data may remain preserved.

---

# 8. Closed

Closed means the account is no longer actively usable.

Possible actions:

* login recovery;
* reactivation;
* data export;
* deletion request.

---

# 9. Deleted

Deletion is the terminal data lifecycle where applicable.

Deletion should occur only after:

* applicable retention requirements;
* grace periods;
* user requests;
* subscription lifecycle;
* dispute requirements.

---

# 10. Subscription Lifecycle

Separate lifecycle:

```text
TRIAL / FREE
↓
ACTIVE
↓
PAYMENT DUE
↓
GRACE
↓
PAST DUE
↓
RESTRICTED
↓
INACTIVE
↓
TERMINATED
```

A Free account may not pass through paid billing states.

---

# 11. Free Does Not Mean Temporary

Free access should not automatically expire.

A user should be able to remain on Free indefinitely, subject to product rules.

---

# 12. Subscription Inactivity

The proposed rule is:

> **Three months of inactive subscription can trigger removal from active capacity.**

This needs a precise definition.

“Inactive subscription” should mean:

* paid plan expired;
* renewal not completed;
* no qualifying reactivation;
* grace period exhausted.

It should not mean:

> user did not open the app for three months.

---

# 13. The Three-Month Window

Conceptual flow:

```text
Subscription expires
       ↓
Grace period
       ↓
1 month
       ↓
2 months
       ↓
3 months
       ↓
Capacity released
```

The actual dates must be communicated clearly to the user.

---

# 14. Capacity Release

When the threshold is reached:

* active subscription allocation ends;
* scarce account capacity is released;
* identifier may return to the allocation pool;
* expensive resources may be reclaimed.

This does not automatically mean immediate destruction of all data.

---

# 15. Identifier Lifecycle

Identifiers may have their own lifecycle:

```text
AVAILABLE
↓
ASSIGNED
↓
ACTIVE
↓
RECLAIMABLE
↓
RELEASED
↓
AVAILABLE
```

If identifiers are scarce, they should not remain permanently reserved for abandoned accounts.

---

# 16. Number Reuse

A released number may eventually be assigned to another user.

Therefore:

> identifiers should not be treated as immutable proof of identity.

Historical records should use stable internal identifiers.

---

# 17. Internal Identity

Circle should maintain an immutable internal identity key distinct from:

* username;
* phone number;
* email;
* public handle;
* account number.

This protects historical relationships if public identifiers change.

---

# 18. Return After Inactivity

A returning user may encounter:

> **Your previous Circle account is no longer active.**

They may:

* create a new active subscription;
* request restoration where available;
* enter the reactivation queue;
* create a new account identity if necessary.

---

# 19. Long List

The proposed long-list mechanism can handle cases where:

> demand exceeds available capacity.

Conceptually:

```text
RETURN REQUEST
      ↓
ELIGIBILITY
      ↓
WAITLIST
      ↓
CAPACITY AVAILABLE
      ↓
ACTIVATION
```

The user should know their status.

---

# 20. Queue Fairness

The queue should have explicit rules.

Possible ordering factors:

* request timestamp;
* previous eligibility;
* capacity availability;
* account state.

Rules must be deterministic and auditable.

---

# 21. Circle Lifecycle

A Circle has its own lifecycle.

```text
DRAFT
↓
ACTIVE
↓
PAUSED
↓
ARCHIVED
↓
DISSOLVED
```

---

# 22. Draft Circle

A Circle has been created but not launched.

Useful for:

* setup;
* invitation;
* configuration;
* capability selection.

---

# 23. Active Circle

The Circle is operational.

Members can participate according to permissions.

---

# 24. Paused Circle

Temporarily inactive.

Examples:

* seasonal organization;
* inactive Fellowship;
* paused Business.

Data remains.

---

# 25. Archived Circle

No active participation.

The Circle remains available for historical reference to authorized users.

---

# 26. Dissolved Circle

The Circle no longer exists operationally.

Examples:

* Business closed;
* Fellowship ended;
* Community organization dissolved.

Historical records may remain subject to privacy and legal rules.

---

# 27. Membership Lifecycle

```text
INVITED
↓
PENDING
↓
ACTIVE
↓
PAUSED
↓
LEFT / REMOVED
```

Leaving does not automatically erase historical participation.

---

# 28. Object Lifecycle

Individual objects may have:

```text
DRAFT
↓
ACTIVE
↓
COMPLETED
↓
ARCHIVED
```

or domain-specific states.

---

# 29. Data Lifecycle

Data should follow:

```text
CREATED
↓
ACTIVE
↓
ARCHIVED
↓
RETENTION
↓
DELETED / ANONYMIZED
```

Different object types can have different retention rules.

---

# 30. Media Lifecycle

Media may have additional states:

* uploaded;
* processed;
* published;
* hidden;
* archived;
* deleted.

A media deletion should not necessarily destroy the Story object referencing it.

The Story can retain:

> media unavailable.

---

# 31. Financial Lifecycle

Financial records have stronger retention requirements.

They may need:

```text
ACTIVE
↓
SETTLED
↓
RECONCILED
↓
RETAINED
```

Deletion may be restricted.

---

# 32. Audit Lifecycle

Audit events should generally be append-only.

They should not be casually edited or deleted.

Corrections should be represented by new events.

---

# 33. Account Suspension

Suspension is different from inactivity.

Suspension may occur because of:

* security concerns;
* abuse;
* fraud investigation;
* legal requirement;
* policy enforcement.

Suspension should not automatically destroy data.

---

# 34. Reinstatement

Where appropriate:

```text
SUSPENDED
↓
REVIEW
↓
REINSTATED
```

or:

```text
SUSPENDED
↓
TERMINATED
```

Actions should be logged.

---

# 35. Death / Succession

Circle's long-term continuity model should eventually support circumstances where an account holder dies or becomes permanently unavailable.

This is particularly important for:

* Family;
* Business;
* Community;
* important shared records.

Possible mechanisms:

* designated successor;
* trusted delegate;
* Circle administrator;
* verified legal process.

This must be designed carefully and separately from ordinary inactivity.

---

# 36. Data After Account Closure

Closure should not automatically destroy shared history.

Example:

A person contributed to a Community project.

If they close their account:

> the Community's historical record may still need to say that a contribution occurred.

But personal information may need to be:

* hidden;
* minimized;
* anonymized;
* retained only where justified.

---

# 37. Reactivation

Reactivation should preserve continuity where possible.

Potential states:

```text
REACTIVATION REQUEST
↓
IDENTITY VERIFICATION
↓
RESTORATION
↓
ACTIVE
```

Restoration should never bypass current security requirements.

---

# 38. Notifications

Users should receive lifecycle warnings before major transitions.

Examples:

> Your subscription expires in 7 days.

> Your account has been inactive for 60 days.

> Your capacity allocation will be released in 30 days.

> Your identifier may become available to another user.

Notifications should be understandable and actionable.

---

# 39. Export Before Deletion

Where appropriate, Circle should offer:

> **Export your data**

before irreversible deletion.

Export should explain:

* what is included;
* what cannot be exported;
* what will remain in shared Circles.

---

# 40. Shared Data Problem

A person's data can become part of someone else's history.

Example:

> A photograph shared in a Family Story.

Deleting the person's account cannot necessarily erase every copy or reference without affecting others.

Circle therefore needs explicit rules for:

* ownership;
* authorship;
* participation;
* shared records;
* personal data.

---

# 41. Lifecycle Event Log

Major transitions should produce events.

Examples:

```text
ACCOUNT_CREATED
SUBSCRIPTION_STARTED
SUBSCRIPTION_EXPIRED
GRACE_STARTED
ACCOUNT_RESTRICTED
CAPACITY_RELEASED
IDENTIFIER_RELEASED
ACCOUNT_CLOSED
ACCOUNT_REACTIVATED
ACCOUNT_DELETED
```

---

# 42. Lifecycle State Machine

The final system should maintain separate state machines for:

```text
Account
Subscription
Circle
Membership
Object
Data
Identifier
```

This prevents one status from incorrectly controlling everything.

---

# 43. Core Principle

Circle should never make the user wonder:

> “What happened to my account?”

The lifecycle must be:

* predictable;
* visible;
* recoverable where possible;
* respectful of shared history;
* explicit about irreversible actions.

---

# 44. Final Model

```text
                 ACCOUNT
                    │
       ┌────────────┼────────────┐
       ↓            ↓            ↓
 SUBSCRIPTION     CIRCLES      IDENTITY
       │            │
       ↓            ↓
 CAPACITY       MEMBERSHIP
                    │
                    ↓
                 OBJECTS
                    │
                    ↓
                   DATA
                    │
                    ↓
              RETENTION / DELETE
```

The key architectural rule is:

> **No single lifecycle should secretly control another.**

That separation is what makes Circle's long-term continuity model safe, understandable, and scalable.
