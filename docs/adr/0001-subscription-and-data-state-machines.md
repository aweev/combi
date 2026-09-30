# ADR-0001: Subscription and Data State Machines

**Status:** Accepted
**Date:** 2026-09-30
**Deciders:** Founder (pending ratification of wording), Docs sync
**Supersedes:** conflicting enumerations in PRD §74, Lifecycle §10, Data Architecture §47, Monetization §22, Pricing §18

## Context

Five documents enumerated the subscription lifecycle with five incompatible
state lists:

| Source | States |
|---|---|
| `PRD.md:1447-1456` (§74) | TRIAL, ACTIVE, PAST_DUE, CANCELLED, EXPIRED, GRACE, INACTIVE, DELETION_PENDING, DELETED |
| `Lifecycle & Account Status.md:153-168` (§10) | TRIAL/FREE, ACTIVE, PAYMENT DUE, GRACE, PAST DUE, RESTRICTED, INACTIVE, TERMINATED |
| `Data Architecture & Technical Foundations.md:673-681` (§47) | TRIAL, ACTIVE, PAST_DUE, CANCELLED, EXPIRED, GRACE_PERIOD, INACTIVE, DELETION_PENDING, DELETED |
| `Monetization & Capacity Strategy.md:462-470` (§22) | PAYMENT DUE, GRACE PERIOD, RESTRICTED, INACTIVE, ARCHIVED |
| `Pricing & Tier Specification.md:358-366` (§18) | PAYMENT FAILED, RETRY, GRACE PERIOD, LIMITED MODE, INACTIVE |

Three names existed for the same state (`GRACE` / `GRACE_PERIOD` / `GRACE
PERIOD`) and three for the degraded state (`RESTRICTED` / `LIMITED MODE` /
`ARCHIVED`). `Lifecycle & Account Status.md` §42 and §44 simultaneously demand
seven *separate* state machines and state that "no single lifecycle should
secretly control another" — the correct principle, but unimplementable while
the enumerations disagree. No billing migration or enum could be written.

## Decision

**One subscription machine, one data machine, one account machine.** `Lifecycle
& Account Status.md` §10 is canonical because it is the most careful (it alone
defines what "inactive" means and warns free accounts skip paid states).

Subscription lifecycle (canonical):

```text
TRIAL
  ↓
ACTIVE
  ↓
PAST_DUE          (payment failed; retries in progress)
  ↓
GRACE             (time-boxed, configurable)
  ↓
RESTRICTED        (Limited Mode: view, communicate, export, renew, reactivate)
  ↓
INACTIVE          (commercial relationship dormant; data intact)
  ↓
CLOSED            (terminal for the commercial relationship)
```

Two additional rules are part of this decision:

1. **A FREE subscription never enters PAST_DUE, GRACE or RESTRICTED-by-payment.**
   Free access does not expire for non-payment (`Lifecycle` §11). Restriction of
   a free account happens only through policy/suspension paths, never billing.
2. **Data lifecycle is separate and named explicitly:**
   `ACTIVE → ARCHIVED → DELETION_PENDING → DELETED`. `RESTRICTED` is a
   *commercial* state and must never appear in the data machine. `DELETION_PENDING`
   is reachable only from explicit user request or a legal requirement — never
   from payment failure (see ADR-0005).

Naming is fixed: `GRACE` (not `GRACE_PERIOD`), `RESTRICTED` with
"Limited Mode" retained only as the user-facing label, `CLOSED` (replacing
`TERMINATED`), `CANCELLED` and `EXPIRED` folded into `PAST_DUE`→`GRACE`
transitions as causes rather than states.

## Consequences

- Positive: one enum can be written; billing code, migrations and telemetry
  share it. The free/paid split is explicit, removing the C-5 contradiction.
- Positive: the "no lifecycle secretly controls another" rule now has a
  concrete machine boundary.
- Negative: `CANCELLED` and `EXPIRED` disappear as stored states; existing
  `api/` financial code (legacy `pkg/domain/financial`) may model them and will
  need mapping during the payments port.
- Negative: user-facing copy must say "Limited Mode" while code says
  `RESTRICTED`, so a label map is required in the localization layer.

## Alternatives Considered

- **Keep `PRD` §74 as canonical** — rejected: it conflates the data machine
  (`DELETION_PENDING`, `DELETED`) into the subscription machine, which is
  exactly what `Lifecycle` §44 forbids.
- **Adopt `Monetization` §22 with `ARCHIVED` as the degraded state** —
  rejected: contradicts its own §22 closing sentence, "critical data should not
  disappear merely because a payment failed."
- **Nine states including `RETRY` as a state** — rejected: retry is a mechanism
  inside `PAST_DUE`, not a lifecycle position.
