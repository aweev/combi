# ADR-0007: Face Taxonomy

**Status:** Accepted
**Date:** 2026-09-30
**Deciders:** Docs sync
**Related:** ADR-0002 (archetypes), `GLOSSARY.md`

## Context

`Domain Model.md:118-138` (§5) and `Identity Privacy & Trust.md` §4-6 both define
Face types as **Personal / Business / Pseudonymous**. `Identity Face & Personal
Space.md` then presents **"Public Face"** (§6, line 101) and **"Private Space"**
(§7, line 120) as if they were peer types, and the same concept — Identity
Verification — is defined twice with no pointer between files (`Identity Privacy
& Trust.md` §23 at line 449 vs `Identity Face & Personal Space.md` §11 at line
192).

If Public/Private are Face *types*, then visibility becomes a kind of identity
rather than a property, which breaks `Product Philosophy` §19 ("Privacy should
not be reduced to one Public/Private switch").

## Decision

**Face types are exactly three: `PERSONAL`, `BUSINESS`, `PSEUDONYMOUS`** (a
fourth, `ROLE-DELEGATED`, is explicitly deferred, not added).

**Public / Private are visibility levels, not types.** `Identity Privacy &
Trust.md` §9 "Visibility Levels" is canonical for visibility; "Public Face" is
renamed to *Face Visibility* and "Private Space" becomes a documented **mode of
a Personal Face** (personal Story/Next/notes that belong to the person rather
than to any Circle — that concept is preserved, its taxonomy demoted).

"Identity Verification" gets one definition (in `Identity & Privacy.md`, the
merged document from Phase 3) and one cross-reference from the other site.

## Consequences

- Positive: visibility becomes orthogonal to identity type, matching the
  contextual-privacy promise and `Pricing`'s archetype/plan decoupling.
- Positive: removes a duplicate definition pair that would have produced two
  verification implementations.
- Negative: `Identity Face & Personal Space.md` §6-7 headings change, so any
  deep links or citations to "Public Face" need updating.
- Note: `Privacy Is Contextual` (`Product Philosophy` §19) remains the governing
  principle and now has a coherent mechanism beneath it.

## Alternatives Considered

- **Four Face types including Public** — rejected: makes "public" an identity
  instead of a disclosure setting; a Personal Face can already be partly public.
- **Treat Private Space as a separate primitive** — rejected: it is a scope rule
  over Story/Next objects owned by a Person, and inventing an object for it
  violates the Expansion Rule (`Core Objects` §41).
