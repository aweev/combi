# Circle Capability Contract

**Status:** AGREED (interface), BLOCKED-BY-CODE (implementation)
**Purpose:** make "secondary tools live inside Circle" a *technical* reality instead
of a UI slogan, resolving the `Product Philosophy` §20 / `Navigation` §32 tension
against hub-and-tab expansion.

**Related:** ADR-0008 (nesting), `Core Objects` §41-43 (Expansion / Anti-Bloat /
Simplicity tests), `PRD` §104, and `api/docs/architecture/service-portability-standard.md`
— existing platform precedent this file deliberately reuses.

---

## 1. The rule

> A capability is **installed into a Circle**. It is never a tab, a sibling product,
> or a separate account surface.

`Core Objects` §42 states this as the Anti-Bloat Rule: never build *"Circle for X"*
when the requirement is *"Circle with capability X"*. This document is the mechanism
that makes that sentence enforceable in code rather than in prose.

## 2. What a capability must declare

Reusing the platform's existing `core.ServiceManifest` fields
(`service-portability-standard.md`) plus Circle-specific additions:

```yaml
service_name:    cargo | inventory | governance | booking | treasury | media | ...
display_name:    Human label (one of the 7 vocabulary terms, or a verb phrase — ADR-0010)
version:         semver of the contract, not the build
tier:            capability            # platform | capability | domain
portable:        true|false
circle_archetypes: [FAMILY, FELLOWSHIP, BUSINESS, COMMUNITY]   # where it is offered
requires:
  context:  [person, face, membership, role, permissions]
  objects:  [story, activity, commitment, event, project, object, document, transaction]
  events:   [circle.member.joined, ...]
contributes:
  to_story: true|false           # writes durable history?
  to_now: true|false             # creates attention/commitments?
  to_next: true|false            # creates intentions?
  to_possibility: true|false
billing:
  ccu_dimensions: [storage, api, transactions]   # ADR-0003
  included_in: [PLUS, ADVANCED, ORGANIZATION]
  metered_above: <allowance>
interfaces:   grpc-first (platform.<capability>.v1); REST for external clients
dependencies: [postgres, redis, events, temporal, storage, payments]
```

## 3. Contract obligations

A capability **must**:

1. **Receive identity and permission context** — never mint its own login, account
   type or permission system (`Core Objects` §43 lists exactly those as the smell
   test that it does not belong in the foundation).
2. **Write into the temporal model.** Anything touching neither Story, Now, Next nor
   Possibility is a separate product, not a capability. This is the gate that keeps
   the "everything app" boundary real rather than rhetorical.
3. **Respect Circle scope.** Data written inside Circle A is invisible to Circle B
   unless an explicit Affiliation discloses it (`Architecture` §24; contextual
   privacy `Product Philosophy` §19).
4. **Declare CCU consumption** so metering is designed in rather than retrofitted
   (ADR-0003).
5. **Be disable-safe.** When `DISABLED` or `RESTRICTED` (`Core Objects` §19), existing
   data must remain readable, exportable and intact.
6. **Be localizable**, including RTL, with no hardcoded user-facing strings
   (`Language` §37, ADR-0009).
7. **Not bypass permissions through translation** (`Language` §33).
8. **Emit events with the standard property set** (`Measurement.md` §5).

A capability **must not**:

* create a new navigation destination (progressive disclosure only, `Navigation` §31-32);
* introduce a new user-facing noun without an ADR-0010 change request;
* hold funds — Vault Level 3 is out of scope (`Vault` §66);
* assume unbounded nesting (depth 1, ADR-0008).

## 4. Interface sketch

```proto
package platform.cargo.v1;

service Capability {
  rpc Install(CircleContext) returns (InstallResult);     // into one Circle
  rpc Uninstall(CircleContext) returns (UninstallResult); // disable-safe
  rpc Describe(Empty) returns (CapabilityManifest);       // section 2 fields
}

message CircleContext {
  string circle_id      = 1;
  string acting_face_id = 2;   // face id, never raw person id
  string archetype      = 3;
  string plan           = 4;
  repeated string roles = 5;
  string locale         = 6;   // en-NG | fr-CM | pcm-NG | ha-NG-Arab
  string trace_id       = 7;
}
```

Events describe facts that already happened; Temporal jobs describe commands that
must be retried until done or dead-lettered (platform convention, already stated in
`service-portability-standard.md`).

## 5. Lifecycle parity

Capability state and Circle state are **separate machines** — ADR-0001's rule that no
lifecycle may secretly control another.

```text
Capability: AVAILABLE -> ENABLED -> DISABLED -> RESTRICTED -> DEPRECATED
Circle:     CREATED   -> ACTIVE   -> DORMANT  -> CLOSED
```

A CLOSED Circle does not delete capability data. A DEPRECATED capability does not
delete a Circle. Export must include capability-owned records in the Circle's export
bundle, because Limited Mode guarantees export survives (Pricing 19).

## 6. The review gate (drop-in for PRD 104)

Before approving any new tool, answer in writing:

1. Which Circle does it belong to, and which archetypes get it?
2. Which of Story / Now / Next / Possibility does it write to?
3. Can existing objects represent it (Core Objects 41, steps 1-3)?
4. What CCU dimensions does it consume, and is that priced (ADR-0003)?
5. Does it need a new primitive, account type, permission system or nav item? If yes
   to any, it is probably **another product** (Product Philosophy 20).
6. What happens to its data when the capability is disabled or the Circle lapses?

Six written answers required. This is the cheapest available defence against the
failure mode the documents already name but had no guardrail for.

## 7. Cast and CONUS under this contract

| Existing thing | Correct classification |
|---|---|
| arena, squads, syndicate, burnbook, cortex, courtroom, creator-studio (7 `services/apps/cast/*`) | capabilities installed into Circles, or Circles of their own archetype - decide per service |
| CONUS shows, quizzes, polls, challenges, voting | capabilities producing Story + Now (participation), monetised by sponsorship, not by Circle plans |
| Cargo / Inventory / Finance / Governance / Booking | capabilities - already named as such in Monetization 6 and Pricing 15 |
| NGO project / campaign / impact / form / matchmaking / event | capabilities for a **Community** or **Business** Circle; an NGO is an archetype setting, not a separate product |

This replaces "everything gets a Circle tab", which Navigation 32 forbids.
