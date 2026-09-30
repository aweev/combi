# Circle Documentation Index

**Purpose:** authority order and dependency graph for the Circle specification set.
Before this index existed the documents contained **zero cross-references to one
another**, which is why thirteen contradiction clusters could coexist indefinitely
(see `../../docs/adr/0000.adr.md`).

Read this first. It says which document wins when two disagree.

---

## 1. Authority order

When two documents conflict, the one with the **lower number wins**, unless an
accepted ADR says otherwise. `Status: Master / Replacement Draft` in `PRD.md` was
previously meaningless because nothing stated what it replaced — it now does.

| # | Document | Authority over | Status |
|---|---|---|---|
| 1 | [PRD.md](PRD.md) | Product definition, MVP scope, intent | AGREED |
| 2 | [Domain Model.md](Domain%20Model.md) | Entities, relationships, states as data | AGREED |
| 3 | [Core Objects & Capability Foundation.md](Core%20Objects%20&%20Capability%20Foundation.md) | Primitive set; Expansion / Anti-Bloat rules | AGREED |
| 4 | [Product Philosophy.md](Product%20Philosophy.md) | Principles, vocabulary budget, hard boundaries | AGREED |
| 5 | [Archetypes.md](Archetypes.md) | Archetypes, circle types, capability composition | AGREED (merged) |
| 6 | [Identity & Privacy.md](Identity%20&%20Privacy.md) | Face taxonomy, privacy, permissions, trust, verification | AGREED (merged) |
| 7 | [Lifecycle & Account Status.md](Lifecycle%20&%20Account%20Status.md) | **Canonical state machines** (ADR-0001) | AGREED |
| 8 | [Capability Contract.md](Capability%20Contract.md) | How a capability installs into a Circle | AGREED |
| 9 | [Architecture.md](Architecture.md) | System structure, branches/affiliations, IA | AGREED |
| 10 | [Data Architecture & Technical Foundations.md](Data%20Architecture%20&%20Technical%20Foundations.md) | Storage, sync, technical constraints | AGREED |
| 11 | [Navigation & Experience Architecture.md](Navigation%20&%20Experience%20Architecture.md) | Navigation, progressive disclosure | AGREED |
| 12 | [Story Now Next Possibility.md](Story%20Now%20Next%20Possibility.md) | Temporal model and the loop | AGREED |
| 13 | [Interconnection & Context Engine.md](Interconnection%20&%20Context%20Engine.md) | Cross-context linking, possibility engine | AGREED |
| 14 | [Search Discovery & Possibility Expansion.md](Search%20Discovery%20&%20Possibility%20Expansion.md) | Search and discovery behaviour | AGREED |
| 15 | [Communication & Notification System.md](Communication%20&%20Notification%20System.md) | Messaging and notification rules | AGREED |
| 16 | [Vault & Financial Model.md](Vault%20&%20Financial%20Model.md) | Financial objects; record/initiate/custody levels | AGREED |
| 17 | [Monetization & Capacity Strategy.md](Monetization%20&%20Capacity%20Strategy.md) | Economic philosophy; billing unit (ADR-0003) | AGREED |
| 18 | [Monetization & Economics.md](Monetization%20&%20Economics.md) | Cost model, revenue architecture, price structure | **BLOCKED-BY-CODE** for numbers |
| 19 | [Pricing & Tier Specification.md](Pricing%20&%20Tier%20Specification.md) | Plan structure, price composition, cancellation | AGREED |
| 20 | [Legal Trust & Safety.md](Legal%20Trust%20&%20Safety.md) | Legal framing, minors, moderation, deletion law | AGREED |
| 21 | [Trust & Abuse Operations.md](Trust%20&%20Abuse%20Operations.md) | Abuse model, rate limits, enforcement ladder | AGREED policy / BLOCKED-BY-CODE |
| 22 | [Measurement.md](Measurement.md) | North Star, events, dashboards, retention | AGREED (V1 metrics) |
| 23 | [Language & Localization.md](Language%20&%20Localization.md) | Locales, translation, RTL (ADR-0009) | AGREED |
| 24 | [Platform Relationship.md](Platform%20Relationship.md) | Combi ↔ Circle ↔ CONUS ↔ Cast hierarchy | Proposed (ADR-0006) |
| 25 | [GLOSSARY.md](GLOSSARY.md) | Controlled vocabulary | AGREED |
| 26 | [IMPLEMENTATION_READINESS.md](IMPLEMENTATION_READINESS.md) | Doc claim vs code reality | AGREED (dated audit) |

## 2. Superseded (do not edit)

| File | Moved to |
|---|---|
| [Archetypes & Circle Types.md](Archetypes%20&%20Circle%20Types.md) | Archetypes.md |
| [Archetypes & Capabilities.md](Archetypes%20&%20Capabilities.md) | Archetypes.md |
| [Identity Privacy & Trust.md](Identity%20Privacy%20&%20Trust.md) | Identity & Privacy.md |
| [Identity Face & Personal Space.md](Identity%20Face%20&%20Personal%20Space.md) | Identity & Privacy.md |

These four were ~2,800 duplicated lines with conflicting enums. Each keeps a redirect
stub naming the conflict it embodied, and merge integrity was verified by asserting
every source line survives in the merged target.

## 3. Decision records

Accepted arbitration lives in `../../docs/adr/` (ADR-0001 … 0010; see
[the index](../../docs/adr/0000.adr.md)). Documents **reference** ADRs instead of
restating rules, so a change happens in exactly one place. ADR-0003/0004/0005/0006 are
still **Proposed** (money gates) — recommendations until signed.

## 4. Status legend

| Status | Meaning |
|---|---|
| **AGREED** | Internally consistent; safe to build from |
| **Proposed** | Awaiting founder sign-off |
| **DRAFT** | Under revision; not settled |
| **SUPERSEDED** | Replaced; retained for history only |
| **BLOCKED-BY-CODE** | Depends on an implementation decision or capability that does not exist yet |

## 5. Verification invariants

Re-checkable after any future edit:

* **one** subscription enum, defined in `Lifecycle & Account Status.md` §10 only;
* **one** archetype enum: Family, Fellowship, Business, Community;
* **one** commercial unit: CCU (`Monetization` §3, `PRD` §73);
* no price that doubles per user milestone;
* no dormant-account deletion, identifier reclamation or waitlist anywhere;
* `grep -c 'id="' *.md` → **0** (98 WYSIWYG artifacts removed);
* every `X.md` reference resolves to an existing file;
* every document has a `Status:` line and appears in this index.
