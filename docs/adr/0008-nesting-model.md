# ADR-0008: Nesting Model — Branches and Affiliations

**Status:** Accepted
**Date:** 2026-09-30
**Deciders:** Docs sync; founder awareness recommended (affects the flagship pitch)
**Supersedes:** "intentional audience layer" (undefined term in `Product Philosophy` §3)

## Context

Circles-within-Circles had four names and two stances:

- `Archetypes & Circle Types.md:514-536` (§27 Nested Circles) — "may eventually
  support nested organizational structures", e.g. Community → Youth Group.
- Same document §28 (540-548) — **"Avoiding Organizational Complexity ... Circle
  should not encourage users to create Circle inside Circle inside Circle merely
  because the software permits it."**
- `Architecture.md:635-657` (§24) — defines the sanctioned mechanisms: **Branches**
  (`PARENT CIRCLE → BRANCH A/B/C`, each branch an independent Circle) and
  **Affiliations** (peer links, `FAMILY ── BUSINESS`, each side sharing only what
  it intentionally exposes).
- `Product Philosophy.md:65-67` — groups should "branch, affiliate, **or create
  an intentional audience layer**". The third mechanism is defined nowhere.
- `Domain Model.md:32` treats CIRCLE as a single primitive with no parent field
  enumerated in §6.

## Decision

**Two nesting mechanisms only, both already specified in `Architecture.md` §24:**

1. **Branch** — parent → child, **depth capped at 1** (a branch is a Circle in
   its own right and cannot itself have branches). Use for organisational units:
   Business → Operations/Marketing/Finance; Community → Youth Group/Sports Group.
2. **Affiliation** — peer-to-peer link between any two Circles, with explicit
   per-side disclosure. Use for relationships, not hierarchies: Family ──
   Business, Community ── Family.

**Unbounded recursion is not supported.** The `Product Philosophy` §3 phrase
"or create an intentional audience layer" is **deleted** (it describes no object
in the domain model). If an audience/broadcast need appears later, it must be
proposed as a capability through the Expansion Rule (`Core Objects` §41), not as
a third nesting shape.

`Domain Model.md` §6 gains a `parent_circle_id` (branch) field and the
Relationship primitive (§5, which already lists `Circle → Circle`) remains the
carrier for affiliations — no new primitive is required, which is the point.

## Consequences

- Positive: permission inheritance becomes computable. Depth-1 means an
  effective-permission check touches at most one parent, avoiding recursive
  resolution in the hot path.
- Positive: contextual privacy (`Product Philosophy` §19) and subject-access
  deletion stay tractable; deep graphs make both a compliance hazard.
- Positive: matches `Archetypes & Circle Types` §28's warning rather than
  fighting it.
- Negative: the headline phrase "Circles within Circles" must be described as
  *branching*, not unlimited nesting, in any launch material.
- Negative: genuinely deep organisations (national association → region →
  chapter → team) cannot be expressed. Accepted; if demand appears, the answer
  should be Affiliation chains plus a Directory capability, not recursion.

## Alternatives Considered

- **Unbounded nesting** — rejected: permission inheritance × contextual privacy
  × lawful deletion across a deep graph is the largest correctness and
  compliance cost in the product, and the spec itself warns against it.
- **Single-level only, no affiliations** — rejected: `Architecture` §24's
  affiliation case (Family ── Business) is a real, distinct need that hierarchy
  cannot express.
- **Keep "intentional audience layer"** — rejected: an undefined term in a
  governing principle document is worse than a missing feature.
