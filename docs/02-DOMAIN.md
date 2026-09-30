# Circle Domain Contract

**Status:** BUILD CONTRACT — the only document engineering builds product logic from.
**Authority:** entities, relationships, states-as-data, permissions, events.
**Supersedes:** `apps/circle/Domain Model.md` (935 lines), `Lifecycle & Account Status.md` (787 —
including its re-entry queue mechanism, deleted by D-10), `Identity & Privacy.md` (1,346),
`Core Objects & Capability Foundation.md`
(910), `Data Architecture & Technical Foundations.md` (921).
**Rule:** §8 is the **single** definition site for every state enum. No other document may restate one.

---

## 1. The one rule

> **Model reality before features. Few primitives, many compositions.**

New capability is expressed by composing existing objects and relationships. A new primitive is the
exception and requires a decision in `01-DECISIONS.md`. The gates that enforce this are §11.

## 2. Object inventory

```text
IDENTITY      Person · Account · Face · Membership · Relationship · Role · Permission · Consent
STRUCTURE     Circle · Branch · Affiliation · Identifier
TEMPORAL      Activity · Story · Now · Next · Possibility
WORK          Commitment · Event · Project · Task · Document · Media · Place · Evidence
MONEY         Money · Vault · Transaction · Contribution · Obligation · Invoice · Expense · Settlement
PLATFORM      Capability · Usage · Capacity · Entitlement · Subscription · Billing Account · Audit Event
```

Only **seven** of these are user-facing nouns (D-05). The rest are architecture and must not surface in
copy unless a user action requires naming them.

## 3. The identity chain

```text
Person ──< Account          one person, access may be several accounts over time (recovery, succession)
Person ──< Face             PERSONAL | BUSINESS | PSEUDONYMOUS (D-02)
Face  ──< Membership >── Circle
```

- **Person** — the underlying human being. Not an archetype (D-01). Distinct from username, account,
  membership and public profile. One Person participates in many contexts.
- **Account** — access: credentials, devices, security, recovery, subscription, lifecycle, data controls.
  **The account is not the person's identity.** This separation is what makes recovery, delegated
  access, succession and closure expressible at all.
- **Face** — a context-specific representation of a Person: name, image, description, visibility,
  relationships, permissions. Every action in analytics and every permission decision is attributed to a
  **Face, never a raw Person id** (§9).
- **Private Space** — a *mode* of a Personal Face: Story/Next owned by the person, belonging to no
  Circle. Not a primitive, not a Face type (D-02).
- **Membership** — the link Face → Circle, carrying role, status, join date, permissions, participation
  history, invitation source, departure state. §8 has its machine.
- **Role** — what someone may do **inside one context**. **Authority never crosses Circles.** A Role
  holder in Circle A has no standing in Circle B, including A's branches — unless explicitly granted there.
- **Relationship** — a first-class edge between two entities (`Person↔Person`, `Person↔Circle`,
  `Circle↔Circle`, `Business↔Community`) with type, role, status, start/end date, visibility, provenance.
  Affiliations (§4) are Relationships, not a new primitive — which is the point.

## 4. Structure

- **Circle** — a meaningful shared collective context. Has archetype (D-01), members, roles,
  relationships, capabilities, activity, Story, Now, Next, optional Vault, governance, settings.
  A Circle is not a group chat and not an audience.
- **Branch** — `parent_circle_id`, **depth 1** (D-03). The branch is a full Circle with its own members,
  roles and permissions.
- **Affiliation** — peer link, each side separately declares what it discloses. Default is **nothing**.
- **Identifier** — `AVAILABLE → ASSIGNED → ACTIVE`; **reclamation prohibited** (D-10). Namespaces may be
  widened if scarcity ever appears; history is never destroyed to make room.
- **Context isolation** — data written inside Circle A is invisible to Circle B unless an Affiliation
  discloses it. This applies to staff and to the platform's own AI (§7, `05-TRUST.md` §4).

## 5. Temporal model

```text
Activity ──(deliberate preservation)──> Story        not every Activity becomes Story
Now      = what is true / active / needs attention   (state)
Next     = what is intended / upcoming               (declared intent, not a forecast)
Possibility = contextual expansion with provenance   (surfaced option, never a decision)
```

**Preservation classes** (the governing rule for what persists):

| Class | Behaviour |
|---|---|
| Ephemeral | May disappear. Never silently promoted. |
| Preserved | Deliberately becomes Story. |
| Financial | Remains auditable per applicable rules. Never silently ephemeral. |
| Personal | Remains under the person's control. |

> **Nothing skippable is silently lost, and nothing ephemeral is silently made permanent.**

**Story is not an archive dump.** It is meaning selected for preservation, and it must stay readable
without becoming a feed (D-05). **State vs history:** current-state projections are derived; the append-only
record is authoritative. Never overwrite history to render a present view.

## 6. Possibility

```text
current context + relationships + capabilities + history = Possibility
```

An interpretation, not merely a row. Obligations:
1. **Carry provenance** — which context/objects produced it, always shown to the person.
2. **Permission-scoped only** — never inferred across Circles without explicit disclosure (D-03, §4).
3. **No silent identity inference** — do not link Faces or deduplicate people into a single profile without consent.
4. **Possibility is not prediction** (Manifest §7.6). It surfaces; the person decides.

## 7. Money objects (the wedge lives here)

```text
Vault          a Circle's financial context. Level 1 = RECORD ONLY (04-ECONOMICS §3)
Transaction    FROM · TO · AMOUNT · CURRENCY · DATE · STATUS · PURPOSE · REFERENCE · EVIDENCE · SOURCE
Contribution   value given toward a shared purpose (monetary | in-kind | recurring | one-time)
Obligation     value someone owes. **The system distinguishes owed from paid.**
Invoice        a request for payment
Expense        money spent for a Circle/Project/Asset, with reimbursement status
Settlement     resolves Obligations: Expense → Amina owes Daniel 40,000 → Settlement (Paid) → Obligation (Completed)
```

Two invariants that are product promises, not code style:
- **A pending transaction is never described as completed.** Status is surfaced as it is.
- **Money is not held.** Level 3 custody is out of scope (D-10). No float, no interchange, no deposit
  economics in any model — a deliberate profit sacrifice that buys the removal of licensing cost and
  regulatory risk.

## 8. State machines — the only definition site

```text
SUBSCRIPTION  TRIAL · ACTIVE · PAST_DUE · GRACE · RESTRICTED · INACTIVE · CLOSED
ACCOUNT       INVITED · CREATED · ACTIVE · INACTIVE · DORMANT · CLOSED · DELETED
DATA          ACTIVE · ARCHIVED · DELETION_PENDING · DELETED
MEMBERSHIP    INVITED · PENDING · ACTIVE · PAUSED · SUSPENDED · LEFT · REMOVED
CIRCLE        CREATED · ACTIVE · DORMANT · CLOSED        (+ DRAFT pre-launch if needed)
CAPABILITY    AVAILABLE · ENABLED · DISABLED · RESTRICTED · DEPRECATED
TRANSACTION   DRAFT · INITIATED · PENDING · CONFIRMED · FAILED · REVERSED · DISPUTED · CANCELLED
INVOICE       DRAFT · ISSUED · VIEWED · PARTIALLY_PAID · PAID · OVERDUE · DISPUTED · CANCELLED
IDENTIFIER    AVAILABLE · ASSIGNED · ACTIVE               (reclamation prohibited — D-10)
```

**Banned as subscription states** (duplicates, or belonging to another machine): `GRACE_PERIOD`,
`TERMINATED`, `ARCHIVED`, `RETRY`, `CANCELLED`, `EXPIRED`, `DELETION_PENDING`, `DELETED`.

Transition rules that are part of the model:
1. `TRIAL → ACTIVE → PAST_DUE → GRACE → RESTRICTED → INACTIVE → CLOSED`.
2. **A FREE subscription never enters `PAST_DUE`, `GRACE` or payment-driven `RESTRICTED`.** Free access
   does not expire for non-payment; restricting a free account happens only through policy/suspension.
3. `RESTRICTED` = Limited Mode: view, communicate with existing members, **export**, renew, reactivate.
   Capacity-intensive *creation* is restricted. Data is intact.
4. `DELETION_PENDING` is reachable **only** from explicit user request or legal obligation — never from
   payment failure (D-10).
5. **"Inactive subscription" means:** paid plan expired + renewal not completed + no qualifying
   reactivation + grace exhausted. It **never** means "did not open the app for three months."
6. `DORMANT` may stop background processing, move storage to cold tiers and reduce capacity consumption —
   **but the data remains preserved and no identifier is reclaimed.**
7. Capability state and Circle state are separate machines. A CLOSED Circle does not delete capability
   data; a DEPRECATED capability does not delete a Circle.

5. Every surfacing and every action on it is measured (`06-MEASUREMENT.md` §4).

## 9. Event contract

Naming follows the code convention in `pkg/domain/core/events.go`:
`<domain>.<object>.<past-tense-action>`.

**Required, and none are emitted today** (`08-STATUS.md` §4):

```text
circle.created                     circle.member.invited        circle.member.rejected
circle.member.joined               circle.member.left
circle.story.entry_created         circle.story.entry_shared
circle.now.item_completed          circle.next.item_created     circle.next.item_completed
circle.possibility.surfaced        circle.possibility.acted
circle.vault.transaction_recorded  circle.vault.contribution_made
circle.vault.obligation_settled    circle.capacity.threshold_reached
subscription.lifecycle.changed     account.lifecycle.changed    data.lifecycle.changed
capability.installed               capability.disabled
```

**Required properties on every event:** `circle_id`, `actor_face_id` (*never* a raw person id),
`archetype`, `plan`, `capacity_band`, `locale`, `region`, `trace_id`, `emitted_at`.
Emit through `events.Emit` so request/tenant/trace metadata come from middleware context.

**Identity rule:** `circle.member.left` and account-lifecycle events carry no personal data beyond
pseudonymous ids — they are the events most likely to outlive the person who triggered them.

**Facts vs commands:** events describe what already happened; workflow jobs are commands retried until
done or dead-lettered (D-08). Never emit an event in order to request work.

## 10. Permission model

- **RBAC inside a context, ABAC across contexts.** A Role grants abilities within one Circle;
  cross-Circle access requires an explicit Affiliation disclosure, evaluated as an attribute rule.
- Effective permission resolution touches **at most one parent** (depth-1 branching, D-03). No recursive
  permission resolution in the hot path — a permanent, deliberate constraint.
- **Delegation and separation of duties** exist for Business/Community Vault flows: the Face that
  initiates a record is distinguishable from the Face that approves it.
- **Translation must never bypass permissions** — a machine-translated object is visible only to Faces
  that could see the original. This rule is load-bearing and stays verbatim.
- Staff and platform systems are subject to context isolation; any staff access to Circle content
  requires a logged, scoped reason (`05-TRUST.md` §4).

## 11. Expansion gates (three tests, in order)

**Expansion Rule** — before any new feature:
1. Can existing objects represent it? 2. Can existing relationships? 3. Can an existing capability?
4. Only if all three fail does the foundation need a new primitive — and then it needs a decision.

**Anti-Bloat Rule** — never build **"Circle for X"** when the requirement is **"Circle with capability X."**

**Simplicity Test** — a new capability must be explainable as *one new ability using existing objects and
relationships.* If it needs new identity, new account type, new permission system, new database model,
new navigation or new billing architecture, question whether it belongs in the foundation at all.

## 12. Data architecture constraints (the ones that actually bind)

1. **Append-only history; derived projections.** Never mutate history to render the present.
   Story/Now/Next are projections over `Activity`, not separate products.
2. **Soft deletion by default.** Hard deletion only on the `DATA` machine path (§8) with a legal basis.
3. **Every money-bearing object carries `evidence` and `source`.** A record that cannot be traced to the
   Face that entered it is not a record.
4. **Tiering replaces deletion:** hot → cold → archive (D-10). Dormancy saves cost, never destroys continuity.
5. **`actor_face_id` on every write**, so an identity can be revoked without erasing shared history.
6. **Analytics retention is separate from product retention:** aggregate rollups indefinite; event-level 13
   months then aggregate-and-drop; identity-linked rows 90 days then pseudonymised; raw AI inference logs
   30 days (provenance, not content). Nothing in this table justifies deleting a Circle's product data.
7. **Export is a product promise, not an admin feature.** A Circle's export bundle must include
   capability-owned records, because Limited Mode guarantees export survives a lapse (§8 rule 3).

## 13. MVP object set — the build list

Implement in this order; nothing outside this list is in scope for the first Circle.

```text
1 Person      2 Account     3 Face (3 types)    4 Circle      5 Membership
6 Role        7 Permission  8 Relationship      9 Activity   10 Story
11 Now        12 Next       13 Commitment       14 Obligation 15 Transaction
16 Contribution 17 Settlement 18 Vault (Level 1) 19 Consent   20 Audit Event
21 Capability (install/disable)  22 Usage/Capacity (CCU metering v0)  23 Subscription + Billing Account
```

Excluded from V1 despite appearing in older specs: Project, Task, Event, Document, Media pipeline,
Place, Invoice, Expense, Budget, Possibility engine (surfacing v0 = rule-based, provenance-bearing only),
biometric, DID, escrow, wallet-custody, marketplace, governance/voting, automation.


