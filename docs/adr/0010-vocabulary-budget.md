# ADR-0010: User-Facing Vocabulary Budget

**Status:** Accepted
**Date:** 2026-09-30
**Deciders:** Docs sync
**Related:** ADR-0002 (archetypes), ADR-0007 (Faces), `GLOSSARY.md`

## Context

`Product Philosophy.md:363-385` (§16, "Simplicity Is an Architectural
Constraint") commits to a small user vocabulary and lists **six** terms:

```text
Circle · Story · Now · Next · Vault · Possibility
```

Meanwhile:

- `Core Objects & Capability Foundation.md:25-39` (§2) enumerates **13**
  foundational objects (PERSON, CIRCLE, RELATIONSHIP, STORY, ACTIVITY,
  COMMITMENT, EVENT, PROJECT, OBJECT, DOCUMENT, MEDIA, TRANSACTION, CAPABILITY).
- `Domain Model.md:45-55` adds 9 supporting entities (PLACE, TIME, MEDIA,
  DOCUMENT, MONEY, ROLE, PERMISSION, EVIDENCE, CAPABILITY).
- `Navigation & Experience Architecture.md:58-90` (§4) exposes **seven** primary
  navigation items: Home, Circles, Story, Now, Next, Possibilities, **Face**.

So the shipped navigation already surfaces a seventh user-facing term — `Face` —
that the philosophy document does not count. The budget was being violated by the
design before any feature was added.

## Decision

**The user-facing vocabulary is seven terms:**

```text
Circle · Face · Story · Now · Next · Vault · Possibility
```

`Face` is added to the `Product Philosophy` §16 list (it is unavoidable: §4 of
Navigation makes it a primary destination, and `Domain Model` §5 makes it the
context-specific identity through which everything else is acted).

**Everything else is internal.** Activity, Commitment, Event, Project, Object,
Document, Media, Transaction, Relationship, Role, Permission, Evidence, Place,
Time, Money, Capability, Possibility-engine internals — all remain architecture
and must not appear in user-facing copy unless a user action requires naming
them. `Product Philosophy` §16's own rule stands: *"Everything else should appear
only when useful."*

Enforcement: `GLOSSARY.md` (authored in this sync) is the controlled list, split
into **User-facing (7)** and **Internal (everything else)**, and
`IMPLEMENTATION_READINESS.md` treats any new user-facing noun as a change request
against this ADR. The Language Test (`PRD` §105) and the Simplicity Test (§107)
become reviewable rather than rhetorical.

## Consequences

- Positive: the "would someone understand this without reading the philosophy
  document?" test (`Product Philosophy` §17) now has a finite vocabulary to test
  against.
- Positive: prevents the `Navigation` §32 "No Feature Dump" failure mode by
  making the budget enforceable, not aspirational.
- Negative: `Face` is an unusual term for a profile/identity surface; the name
  is retained deliberately because it already appears in 5 documents and in
  navigation, and because it carries the multiple-context model better than
  "profile".
- Note: `Core Objects` §43's Simplicity Test and §42's Anti-Bloat Rule remain the
  architectural gates; this ADR is the *copy and IA* gate.

## Alternatives Considered

- **Keep six terms, demote Face from navigation** — rejected: `Identity Face &
  Personal Space` §26 makes Face the landing page and §21 explicitly rejects a
  follower economy; Face is load-bearing, not decorative.
- **Admit all 13 core objects into the vocabulary** — rejected: this is precisely
  the "collection of disconnected productivity tools" outcome `PRD` §6 forbids.
- **No budget (ship whatever terms emerge)** — rejected: forfeits the one
  differentiation claim (`PRD` §95) that is defensible against incumbents.
