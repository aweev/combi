# Capability Contract

**Status:** INTERFACE SETTLED · BLOCKED-BY-CODE (implementation).
**Purpose:** make "tools live inside Circle" a *technical* reality instead of a UI slogan.
**Authority:** how anything installs into a Circle. Depends on D-03, D-05, D-08, D-11.

---

## 1. The rule

> A capability is **installed into a Circle**. It is never a tab, never a sibling product, never a
> separate account surface.

This is the Anti-Bloat Rule (`02-DOMAIN.md` §11) expressed as an interface, so it can be enforced in code
review rather than argued in prose.

## 2. Manifest

Reuse the platform's own `core.ServiceManifest` (`pkg/domain/core/manifest.go`), plus Circle additions:

```yaml
service_name:    vault | cargo | inventory | governance | booking | media | ...
display_name:    one of the 7 user-facing nouns, or a verb phrase (D-05)
version:         semver of the CONTRACT, not the build
tier:            capability
portable:        true
circle_archetypes: [FAMILY, FELLOWSHIP, BUSINESS, COMMUNITY]   # where it is offered
requires:
  context:  [person, face, membership, role, permissions]
  objects:  [activity, story, commitment, transaction, evidence]
  events:   [circle.member.joined, ...]
contributes:
  to_story: true | false        # writes durable history?
  to_now: true | false          # creates attention or commitments?
  to_next: true | false         # creates intentions?
  to_possibility: true | false
billing:
  ccu_dimensions: [storage, api, transactions]   # D-11
  included_in: [PLUS, ADVANCED, ORGANIZATION]
  metered_above: <allowance>
interfaces:   grpc-first (platform.<capability>.v1); REST for external clients
dependencies: [postgres, redis-jobs, events, temporal?, storage, payments?]
```

## 3. A capability MUST

1. **Receive identity and permission context.** Never mint its own login, account type or permission
   system — needing one is the smell test that it is actually a separate product.
2. **Write into the temporal model.** Touching none of Story / Now / Next / Possibility disqualifies it.
   This is the gate that keeps the "everything app" boundary real.
3. **Respect Circle scope.** Invisible to other Circles unless an Affiliation discloses it.
4. **Declare CCU consumption at design time**, not retrofitted.
5. **Be disable-safe.** When `DISABLED` or `RESTRICTED`, existing data stays readable, exportable, intact.
6. **Be localizable including RTL**, with no hardcoded user-facing strings (D-04).
7. **Never bypass permissions through translation** (`02-DOMAIN.md` §10).
8. **Emit events with the standard property set** (`02-DOMAIN.md` §9).

## 4. A capability MUST NOT

- create a new navigation destination (progressive disclosure only);
- introduce a new user-facing noun without editing D-05;
- **hold funds** — Vault Level 3 is out of scope;
- assume nesting deeper than 1;
- store a duplicate copy of Person / Face / Membership / Role — it references them.

## 5. Interface

```proto
package platform.vault.v1;

service Capability {
  rpc Install(CircleContext) returns (InstallResult);      // into exactly one Circle
  rpc Uninstall(CircleContext) returns (UninstallResult);  // disable-safe, never deletes
  rpc Describe(Empty) returns (CapabilityManifest);        // §2 fields
}

message CircleContext {
  string circle_id      = 1;
  string acting_face_id = 2;   // face id, never a raw person id
  string archetype      = 3;   // FAMILY | FELLOWSHIP | BUSINESS | COMMUNITY
  string plan           = 4;   // FREE | PLUS | ADVANCED | ORGANIZATION
  repeated string roles = 5;
  string locale         = 6;   // en-NG | fr-CM | pcm-NG | ha-NG-Arab
  string trace_id       = 7;
}
```

## 6. Lifecycle parity

Capability state and Circle state are **separate machines** (D-06):

```text
Capability: AVAILABLE → ENABLED → DISABLED → RESTRICTED → DEPRECATED
Circle:     CREATED   → ACTIVE   → DORMANT  → CLOSED
```

A CLOSED Circle does not delete capability data. A DEPRECATED capability does not delete a Circle.
Export includes capability-owned records, because Limited Mode guarantees export survives a lapse.

## 7. Review gate — six written answers before approval

1. Which Circle does it belong to, and which archetypes get it?
2. Which of Story / Now / Next / Possibility does it write to?
3. Can existing objects represent it (`02-DOMAIN.md` §11, steps 1–3)?
4. What CCU dimensions does it consume, and is that priced (D-11)?
5. Does it need a new primitive, account type, permission system or nav item? **Any "yes" means it is
   probably another product.**
6. What happens to its data when the capability is disabled, or the Circle lapses?

Six answers, in writing, in the PR. This is the cheapest available defence against the failure mode the
older documents named repeatedly and had no guardrail for.

## 8. Classifying everything that already exists

| Existing thing | Correct classification |
|---|---|
| `vault` (contribution + obligation + settlement) | capability — **first to ship, Level 1 record only** |
| cargo, inventory, finance, governance, booking | capabilities — already named as such |
| arena, squads, syndicate, burnbook, cortex, courtroom, creator-studio (7 cast services, 7 Go files) | collapse to **one** Cast app or a set of capabilities (D-15) |
| CONUS shows, quizzes, polls, challenges, voting | capabilities producing Story + Now, monetised by **sponsorship**, not Circle plans (07-CONUS) |
| NGO project / campaign / impact / form / matchmaking / event (46 Go files) | capabilities for a **Community** or **Business** Circle. An NGO is an archetype setting, not a product line |
| `api-gateway`, `event-bus-service` | not capabilities — topology artifacts, deleted (D-07, D-09) |

This replaces "everything gets a Circle tab," which was forbidden in prose and possible in code. Now it is
impossible without failing §7.
