# CIRCLE

## Communication & Notification System

### Helping people stay connected without creating noise

---

# 1. Purpose

Communication is central to Circle.

But Circle should not become another noisy messaging platform.

The communication system should connect:

* people;
* Circles;
* Story;
* Now;
* Next;
* Possibilities;
* objects;
* commitments.

---

# 2. Core Principle

> **Communication should have context.**

A message should usually be connected to something meaningful.

Examples:

> a Story;

> a Project;

> an Event;

> a Commitment;

> a Person;

> a Circle.

---

# 3. Communication Types

Circle should support multiple forms:

```text
DIRECT
CIRCLE
OBJECT
EVENT
PROJECT
SYSTEM
```

---

# 4. Direct Communication

Person-to-person communication.

Examples:

* message;
* voice;
* media;
* shared object.

The relationship remains the context.

---

# 5. Circle Communication

Group communication within a Circle.

It should support:

* announcements;
* discussion;
* updates;
* questions;
* decisions.

---

# 6. Object Communication

A conversation attached to an object.

Example:

> Business → Project → Water System → Discussion

This is preferable to discussing the project in a completely separate chat where context can be lost.

---

# 7. Event Communication

An Event can have its own communication context.

Example:

> Family → Event → Wedding

Participants can discuss:

* logistics;
* attendance;
* contributions;
* updates.

---

# 8. Project Communication

Project communication should remain attached to the Project.

This means new members can understand:

> what was discussed;

> why decisions were made;

> what happened.

---

# 9. System Communication

System-generated communication includes:

* invitations;
* reminders;
* payment notices;
* security alerts;
* lifecycle notices.

These must be visually distinguishable from human communication.

---

# 10. Messaging vs Activity

These must remain separate.

### Message

Someone intentionally communicated.

### Activity

Something happened in the system.

Example:

> "Amina said the meeting is postponed."

is a message.

> "Meeting time changed from 10:00 to 14:00."

is an activity.

---

# 11. Notification

A notification is an attention signal.

It is not necessarily a message.

---

# 12. Notification Categories

Possible categories:

```text
PEOPLE
CIRCLES
COMMITMENTS
EVENTS
PROJECTS
TRANSACTIONS
SECURITY
SYSTEM
```

---

# 13. Notification Priority

Notifications should have priority.

### Critical

Security, financial or account issues.

### Important

Commitments, direct requests, important Circle actions.

### Informational

Routine updates.

The system should not treat everything as urgent.

---

# 14. Notification Intelligence

Circle should eventually understand:

> "Does this actually require this person's attention?"

If not:

> aggregate.

---

# 15. Notification Bundling

Instead of:

> 17 notifications

Circle might show:

> **Business**
>
> 17 updates across 3 projects.

The user can expand when needed.

---

# 16. Quietness

Circle should actively protect attention.

Users should be able to configure:

* quiet hours;
* muted Circles;
* muted objects;
* notification categories;
* digest frequency.

---

# 17. Contextual Muting

Users should be able to mute:

> this Project

without muting:

> the entire Business.

This preserves control.

---

# 18. Mentions

Mentions should be meaningful.

Mentioning someone should create attention only when appropriate.

---

# 19. Commitments

A message can create a Commitment.

Example:

> "I'll deliver this Friday."

Circle may ask:

> **Turn this into a commitment?**

The user decides.

---

# 20. Message → Action

Communication can generate:

* Commitment;
* Event;
* Task;
* Project;
* Story;
* Possibility.

This is one of Circle's strongest interconnected features.

---

# 21. Action → Communication

The reverse should also work.

Example:

> Project completed.

Circle can optionally create:

> Story.

Or notify relevant members.

---

# 22. Story From Communication

Meaningful conversations can become Story.

Example:

> Family discussing grandmother's childhood.

Someone can preserve the conversation as:

> Family Story.

The original context remains available according to permissions.

---

# 23. Communication and Privacy

A private message should not become public Story automatically.

The system must respect:

* participants;
* consent;
* Circle visibility;
* object permissions.

---

# 24. Presence

Circle may show whether someone is:

* online;
* recently active;
* unavailable.

But presence should remain optional and privacy-conscious.

---

# 25. Read States

Read states may exist for:

* direct messages;
* critical notifications;
* important Circle announcements.

They should not become social pressure mechanisms.

---

# 26. Reactions

Reactions can provide lightweight feedback.

But they should not become the primary measurement of social value.

Avoid designing Circle around:

> likes.

---

# 27. Replies

Replies should preserve context.

A reply to:

> Project → Budget

should remain associated with:

> Project → Budget.

---

# 28. Threads

Threads may be useful for complex discussions.

But threads should not be mandatory.

The default experience should remain simple.

---

# 29. Announcements

Circle administrators can create announcements.

Announcements should be distinct from ordinary messages.

They may support:

* acknowledgement;
* expiry;
* priority;
* audience;
* attachment.

---

# 30. Decisions

Communication can eventually lead to a formal Decision.

Example:

> "Should we move the event to Saturday?"

The Circle can create a Decision object.

The final outcome becomes part of history.

---

# 31. Notification Sources

Every notification should be traceable to a source object.

Example:

```text
Notification
 ↓
Project
 ↓
Business
 ↓
Person
```

This allows the user to understand why they received it.

---

# 32. Notification Center

The Notification Center should group by context rather than simply by timestamp.

Example:

```text
TODAY

Family
  2 updates

Business
  4 updates

Fellowship
  1 invitation
```

---

# 33. Notification Actions

Notifications should support direct actions where safe.

Example:

> Invitation → Accept / Decline

> Payment request → Review

> Event invitation → RSVP

This reduces unnecessary navigation.

---

# 34. Digest

Users may choose:

* instant;
* hourly;
* daily;
* weekly.

The system should intelligently recommend digesting low-priority activity.

---

# 35. Email / External Notifications

Important events may be delivered through:

* email;
* push;
* SMS;
* supported messaging integrations.

The user should control channels.

---

# 36. No Notification Spam

Circle should have an explicit:

> **Notification Budget**

principle.

The system should prefer:

> fewer meaningful notifications

over:

> maximum engagement.

---

# 37. Anti-Engagement Principle

Circle should never intentionally generate notifications simply to make people open the app.

Notifications should correspond to real events or meaningful opportunities.

---

# 38. Communication Search

Users should be able to search conversations by:

* person;
* Circle;
* object;
* date;
* text;
* event.

---

# 39. Communication History

Important communication should remain connected to the relevant context.

Example:

A Business project completed in 2029 should still be able to show:

> the key discussions that led to the decision.

---

# 40. Communication Lifecycle

Messages can have states:

```text
SENT
DELIVERED
READ
EDITED
DELETED
```

Deletion should respect:

* legal requirements;
* retention rules;
* shared context;
* audit needs.

---

# 41. Safety

The communication system should support:

* blocking;
* reporting;
* moderation;
* spam detection;
* abuse controls.

These should remain separate from ordinary relationship management.

---

# 42. Human Control

Automation should assist communication, not impersonate people without clear disclosure.

AI-generated messages should be identifiable when appropriate.

---

# 43. Communication and Possibility

A conversation may surface a Possibility.

Example:

> "We should collaborate sometime."

Circle might recognize:

> potential collaboration.

But it should ask before creating or promoting that possibility.

---

# 44. Communication and Next

A conversation can become Next.

Example:

> "Let's meet next month."

Circle can suggest:

> **Create an event?**

Again, suggestion first.

---

# 45. Final Communication Model

```text
MESSAGE
   │
   ├──→ ACTIVITY
   ├──→ COMMITMENT
   ├──→ EVENT
   ├──→ PROJECT
   ├──→ STORY
   └──→ POSSIBILITY
```

The user chooses which transitions occur.

---

# 46. Final Principle

> **Circle should help people turn communication into continuity, not turn every interaction into engagement.**
