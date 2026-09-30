# CIRCLE

## Navigation & Experience Architecture

### Making a powerful system feel simple

---

# 1. Purpose

Circle may eventually contain enormous amounts of information and capability.

The user's experience must nevertheless remain simple.

The fundamental UX principle is:

> **Circle should expose context, not complexity.**

Users should not need to understand the architecture to use the product.

---

# 2. The Core Question

Every time someone opens Circle, the interface should help answer:

> **What matters to me right now?**

Not:

> "Which module do you want to open?"

---

# 3. Primary Experience Model

Circle should revolve around:

```text
ME
│
├── NOW
├── STORY
├── NEXT
├── POSSIBILITIES
│
└── CIRCLES
     ├── Family
     ├── Business
     ├── Fellowship
     └── Community
```

This becomes the mental model of the product.

---

# 4. The Main Navigation

A simple global navigation can eventually contain:

### Home

The contextual starting point.

### Circles

Everything the person belongs to or manages.

### Story

Meaningful history and preserved moments.

### Now

Current activity, commitments and attention.

### Next

Intentions, plans and upcoming direction.

### Possibilities

Exploration and opportunity.

### Face

The person's identity and personal landing page.

The exact navigation treatment may differ between desktop and mobile.

---

# 5. Home

Home should not simply be a feed.

It is the **orientation layer**.

It can surface:

* important Now items;
* upcoming events;
* pending commitments;
* recent meaningful Story;
* Circle updates;
* people requiring attention;
* relevant Possibilities.

---

# 6. Home Is Contextual

The Home experience should change according to:

* who the person is;
* which Circles they belong to;
* what is happening;
* what requires attention;
* what they recently interacted with.

Two people should not necessarily see the same Home.

---

# 7. Home Is Not the Feed

Traditional social platforms often begin with:

> "Here are posts."

Circle should begin with:

> **"Here is what matters in your world."**

This distinction is foundational.

---

# 8. The Now Surface

Now answers:

> **What is happening?**

It can contain:

* active commitments;
* upcoming events;
* recent Circle activity;
* pending requests;
* tasks;
* transactions requiring attention;
* current projects.

---

# 9. Now Should Be Time-Sensitive

Now should prioritize things according to:

* urgency;
* relevance;
* relationship;
* commitment;
* deadline;
* importance.

It should not simply sort everything chronologically.

---

# 10. Story

Story answers:

> **What has happened?**

Story may contain:

* personal history;
* Circle history;
* events;
* memories;
* achievements;
* projects;
* milestones.

---

# 11. Story Is Not a Social Feed

Story should be:

* contextual;
* meaningful;
* preservable;
* navigable;
* connected to people and places.

The goal is not endless scrolling.

---

# 12. Next

Next answers:

> **What are we moving toward?**

It can contain:

* goals;
* plans;
* commitments;
* projects;
* upcoming events;
* intentions.

---

# 13. Next Is Action-Oriented

Story looks backward.

Now looks at the present.

Next looks forward.

This temporal structure gives Circle a simple mental model.

---

# 14. Possibilities

Possibilities answers:

> **What else could happen?**

This is the exploratory layer.

It can surface:

* people;
* opportunities;
* ideas;
* projects;
* connections;
* resources;
* capabilities;
* possible next steps.

---

# 15. Possibilities Must Remain Optional

Circle should never silently convert:

> possibility

into:

> recommendation

into:

> commitment.

The user controls the transition.

---

# 16. Face

Face answers:

> **Who am I?**

It provides:

* identity;
* personal Story;
* current context;
* selected Next;
* selected Possibilities;
* relationships;
* Circles.

---

# 17. Circle Home

Each Circle has its own Home.

The Circle Home answers:

> **What is happening in this Circle?**

It can contain:

* Now;
* people;
* Story;
* Next;
* capabilities;
* documents;
* events;
* activity.

---

# 18. Circle Navigation

Inside a Circle, navigation should remain contextual.

Example:

### Family

```text
Home
People
Story
Events
Vault
Next
```

### Business

```text
Home
People
Customers
Projects
Finance
Operations
Story
```

The underlying architecture remains the same.

---

# 19. Context Breadcrumb

Whenever someone enters deeper functionality, Circle should preserve context.

Example:

> Circle → Family → Story → Grandmother's 80th

or:

> Circle → Business → Project → Water System

The user should always know:

> **where am I?**

---

# 20. Context Switching

A user may belong to many Circles.

Switching should be easy.

Example:

```text
ME
│
├── Family
├── Business
├── Fellowship
└── Community
```

Selecting a Circle changes the context without requiring another account.

---

# 21. The Circle Switcher

The Circle switcher should show:

* Face / Me;
* recent Circles;
* favorite Circles;
* all Circles;
* invitations.

It should not become a complicated organization tree.

---

# 22. Quick Actions

Circle should provide contextual quick actions.

Examples:

> Add Story

> Create Event

> Add Commitment

> Start Project

> Invite Person

> Record Transaction

The actions available depend on context.

---

# 23. Universal Create

A universal create action can eventually allow:

```text
Story
Event
Commitment
Project
Document
Transaction
Possibility
```

But the interface should prioritize the actions relevant to the current context.

---

# 24. Deep Linking

Every meaningful object should be directly addressable.

Examples:

* Story;
* Person;
* Circle;
* Project;
* Event;
* Document.

This enables:

* sharing;
* notifications;
* search;
* references;
* history.

---

# 25. Object Pages

An object page should answer:

1. What is this?
2. Why am I seeing it?
3. Who is involved?
4. What happened?
5. What happens next?
6. What can I do?

---

# 26. Relationship Context

When viewing an object, Circle should explain relevant relationships.

Example:

> "This project belongs to the Community Circle and involves 12 members."

This makes the interconnected model visible without exposing database complexity.

---

# 27. Desktop Architecture

Desktop can support:

```text
GLOBAL NAV
      │
CONTEXT NAV
      │
MAIN CONTENT
      │
CONTEXTUAL PANEL
```

This allows powerful workflows without losing orientation.

---

# 28. Mobile Architecture

Mobile should prioritize:

* Home;
* Now;
* Circles;
* Create;
* Notifications.

Secondary functionality can live within context menus and Circle navigation.

---

# 29. Responsive Principle

Desktop can expose more simultaneously.

Mobile should not merely shrink desktop.

The information hierarchy must remain the same.

---

# 30. Empty States

Empty states should teach.

Instead of:

> "No projects."

Use:

> **Nothing here yet.**

Then:

> "Projects help you organize work toward an outcome."

with:

> **Create a project**

---

# 31. Progressive Disclosure

New users should see the smallest useful experience.

As they create:

* Circles;
* Story;
* Events;
* Projects;

Circle gradually reveals relevant functionality.

---

# 32. No Feature Dump

The home screen should not contain:

* 30 icons;
* every capability;
* every metric;
* every module.

Capabilities should appear when context makes them useful.

---

# 33. Search Everywhere

Search should be globally accessible.

The user should not have to know which Circle contains something before searching.

---

# 34. Notifications Everywhere

Notifications should deep-link directly to context.

Example:

> "Daniel commented on your Family Story."

Opening it should take the user directly to:

> Family → Story → specific item.

---

# 35. Universal Context

Every screen should preserve:

```text
WHO
WHERE
WHAT
WHY
WHAT NEXT
```

This is one of Circle's strongest UX principles.

---

# 36. The Orientation Rule

At any moment, the user should be able to answer:

> Who am I acting as?

> Which Circle am I in?

> What am I looking at?

> Why does it matter?

> What can I do next?

---

# 37. Navigation Principle

Circle should feel less like:

> operating software

and more like:

> **moving through your world.**

---

# 38. Final Architecture

```text
                         ME / FACE
                            │
        ┌───────────────────┼───────────────────┐
        ↓                   ↓                   ↓
       NOW                STORY               NEXT
        │                   │                   │
        └───────────────────┼───────────────────┘
                            ↓
                       POSSIBILITIES
                            │
                            ↓
                         CIRCLES
                            │
        ┌─────────────┬─────┼─────┬─────────────┐
        ↓             ↓           ↓             ↓
      FAMILY       BUSINESS   FELLOWSHIP     COMMUNITY
        │             │           │             │
        └─────────────┴───────────┴─────────────┘
                            ↓
                       CAPABILITIES
```

The interface remains simple because the complexity exists in the relationships underneath, not in the navigation itself.

---

# 39. Final Principle

> **Circle should never make users navigate the architecture. Circle should navigate the architecture for them.**
