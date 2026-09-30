# ADR-0005: Dormancy, Payment Failure and Data Loss

**Status:** Proposed — **money gate, requires founder sign-off**
**Date:** 2026-09-30
**Deciders:** Founder (pending)
**Supersedes:** `PRD.md` §75's flattened removal rule, `Monetization` §26 long-list mechanism

## Context

Five documents gave conflicting answers about what happens to a dormant or
unpaid account:

- `Lifecycle & Account Status.md` §11: **"Free Does Not Mean Temporary"** — free
  access should not expire.
- `Lifecycle & Account Status.md` §12: carefully defines "inactive
  subscription" as *paid plan expired + renewal not completed + no qualifying
  reactivation + grace exhausted*, and explicitly says it **must not** mean
  "user did not open the app for three months."
- `PRD.md:1460-1474` (§75): flattened the rule into
  `SUBSCRIPTION ENDS → INACTIVE → 3 MONTHS → ACCOUNT REMOVAL → CAPACITY
  RELEASED`, with **no paid/free guard** — silently overriding §12.
- `Pricing & Tier Specification.md` §19: contradicts removal entirely — expired
  users retain view, communicate, export, renew and reactivate in **Limited
  Mode**.
- `Monetization & Capacity Strategy.md` §22: "critical data should not disappear
  merely because a payment failed."
- `Monetization` §23-26 + `PRD` §77: add identifier release and a
  **long-list / waitlist** re-entry mechanism. §26 appears in exactly one
  document and is referenced by no other.

The product's stated differentiator is continuity (`Product Philosophy` §2
"Preserve continuity", `PRD` §98 North Star "meaningful continuity created").
Deleting a person's accumulated relationship history and then making them queue
to return attacks the one thing the product sells.

## Decision (recommended)

**No data loss and no identifier recycling on payment failure or dormancy.**

1. **Free accounts never expire and never lose data.** `Lifecycle` §11 adopted
   as absolute.
2. **A paid subscription that lapses degrades to `RESTRICTED` (Limited Mode)**
   per `Pricing` §19 and ADR-0001: view, communicate with existing members,
   export, renew, reactivate. Creation of capacity-intensive objects is
   restricted. `INACTIVE` then `CLOSED` end the *commercial* relationship only.
3. **Deletion requires an explicit user request or a legal obligation.** It is
   never a consequence of non-payment or dormancy.
4. **Identifiers are not reclaimed for dormancy** and the **long-list / waitlist
   mechanism is deleted**. If the identifier namespace ever becomes genuinely
   scarce, that is solved by expanding the namespace (ADR-0003 already removes
   the billing incentive to ration it).
5. **Capacity release is about *expensive resources*, not identity.** Dormancy
   may stop background processing and move storage to cold tiers (`Lifecycle`
   §7 Dormant: "reduce background processing, stop expensive services, reduce
   active capacity consumption... but the data may remain preserved") — the cost
   saving without the harm.
6. `PRD` §75 is rewritten to inherit `Lifecycle` §12's paid-only definition and
   §77 is rewritten to Limited Mode reactivation.

## Consequences

- Positive: removes the largest self-inflicted retention and reputation hazard
  in the set. A lapsed customer returns to intact history instead of a queue.
- Positive: eliminates a class of consumer-protection and data-protection risk
  (deleting records because someone could not pay).
- Positive: cost is bounded — cold storage of dormant data is far cheaper than
  the support and churn cost of destroyed continuity.
- Negative: a dormant-data tail that must be priced into storage economics;
  requires a tiering policy (hot → cold → archive) rather than deletion.
- Negative: forfeits the "clean inactive capacity" mechanism that §24-26 assumed
  would keep the namespace compact.
- Risk: indefinite retention conflicts with data-minimisation expectations
  (`Lifecycle` §36 anonymisation, `Legal Trust & Safety`). The retention schedule
  authored in this sync must reconcile "never delete for non-payment" with "do
  not hoard personal data indefinitely" — resolution: anonymise, don't erase.

## Alternatives Considered

- **Keep the 3-month removal rule** — rejected: it contradicts `Lifecycle` §12,
  `Pricing` §19, `Monetization` §22 and the North Star, and it punishes normal
  human dormancy (trips end, weddings pass, projects complete).
- **Remove only the identifier recycling, keep account removal** — rejected:
  account removal *is* the harm; recycling is just its second-order effect.
- **Keep the waitlist but soften it** — rejected: an orphan mechanism described
  in one document, whose "exact mechanics need later product decisions" (§26),
  and which monetises scarcity that ADR-0003 already decided not to create.
- **Charge for storage of dormant paid data** — deferred: defensible for large
  ORGANIZATION accounts, unacceptable for free/personal ones. Belongs in the
  economics model, not in the lifecycle.
