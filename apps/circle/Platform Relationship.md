# Combi Platform Relationship — Circle, CONUS, Cast, NGO

**Status:** Proposed (depends on ADR-0006, which needs founder sign-off)
**Purpose:** the four product identities in this workspace previously existed in
isolation. All 12 CONUS documents contain **zero** mentions of Circle, and all 21
Circle documents contain zero mentions of CONUS or Cast. `plan.md:40` had no
service category for Circle at all. This document states the intended relationship.

---

## 1. The hierarchy

```text
COMBI  (company + monorepo: github.com/combi/*, DB "combi", events "combi:")
  |
  +-- CIRCLE  (flagship: identity, relationships, Circles, Story/Now/Next,
  |            permissions, capabilities, billing)
  |     ^
  |     |  consumes Circle primitives via the Capability Contract
  |     |
  |     +-- CONUS   (participation / media product)
  |     +-- CAST    (application family: arena, burnbook, cortex, courtroom,
  |     |            creator-studio, squads, syndicate)
  |     +-- TOOLS   (capabilities installed into Circles: Cargo, Inventory,
  |                  Governance, Booking, Finance)
  |
  +-- PLATFORM  (services/platform/*: auth, identity, payments, storage, ...)
  +-- LEGACY    (api/ : the "NGO Intelligence Platform" codebase being ported in)
```

## 2. What each layer owns

| Layer | Owns | Must not own |
|---|---|---|
| Combi | brand, monorepo, shared `pkg/`, infra, one identity system | product behaviour |
| Circle | Person, Face, Relationship, Circle, Membership, Role, Permission, Story/Now/Next/Possibility, Billing account | feature-specific logic that belongs to a capability |
| Capability (in Circle) | one job: cargo, inventory, governance, booking... | its own login, account type, permission model, nav item |
| CONUS | shows, quizzes, challenges, sponsorship, reward economy, creator pipeline | a second identity system or its own membership model |
| Cast apps | interactive formats and competitions | duplicating Circle membership or payments |
| Platform | auth, storage, payments, notification, search, realtime, audit | product vocabulary |

## 3. Why this framing is the profitable one

1. **One identity graph, many products.** Each new product reuses Person/Face/
   Membership/Role/Permission instead of rebuilding auth, RBAC and profile. The
   marginal cost of a second product drops to its own logic.
2. **Cross-subsidy without paywalling people.** `Monetization` 16 lets Business and
   organization revenue fund free participation. CONUS carries that logic already:
   its own document says it is *not fundamentally a subscription business*
   (`Business & Economic Model.md:7`) and monetises **sponsorship first**. Circle
   monetises **infrastructure first**. The two revenue engines do not compete.
3. **Retention compounds.** A CONUS interaction can write a Circle Story; a Circle
   commitment can surface a CONUS opportunity. Shared primitives mean engagement in
   one product strengthens the other, which no tab structure can achieve.
4. **One billing stack.** `PRD` 79's entity model (PERSON / CIRCLE / BILLING ACCOUNT
   / PLAN / SUBSCRIPTION / ENTITLEMENT / USAGE / CAPACITY) serves every product.
   CONUS subscriptions (Daily 100 FCFA, Pro 1,000 FCFA) become plans in the same
   engine rather than a second payment integration.

## 4. What is NOT being claimed

* Circle is **not** a portal. `Product Philosophy` 20 forbids the everything-app and
  `Navigation` 32 forbids the feature dump. Integration is by contract, not by icon.
* CONUS's founder ladder stays **earned**, not purchased: "a creator can become a
  founder. a founder can become a partner" (`CONUS Constitution.md:301-303`). It is
  not a payment tier, and must not be conflated with the Founding Circle charter in
  `Monetization & Economics.md` 7.
* NGO is history, not a roadmap. `plan.md` Phase 3 still ports seven `apps/ngo/*`
  services; under this model they land as **capabilities for Community/Business
  Circles**, not as a parallel product line.

## 5. Consequences to accept

1. Circle must ship the primitives CONUS needs (identity, membership, events,
   rewards hooks) or CONUS keeps building its own and the argument dies.
2. A shared identity system means a shared blast radius: one auth incident harms
   every product. `Trust & Abuse Operations.md` becomes a platform document.
3. Brand clarity work: `api/README.md` and `api/config.yml` still say
   "NGO Intelligence Platform" / `ngo-platform.org`. Two of those values (JWT issuer,
   database name) are breaking to change - see ADR-0006.
4. Sequencing: Circle first as platform, CONUS as its first tenant, Cast capabilities
   behind the contract. Launching all four as siblings is what produced four sets of
   documents that never referenced each other.

## 6. Open questions

1. Does CONUS adopt Circle identity now, or keep its own until Circle primitives are
   stable? (Recommendation: keep, but define the seam so the migration is mechanical.)
2. Is Cast one product or seven? `services/apps/cast/*` has 7 directories with 7 Go
   files total - effectively unbuilt. Cheapest answer: one Cast app, several capabilities.
3. Which brand faces users - Combi, Circle, or both (Combi: Circle)?
