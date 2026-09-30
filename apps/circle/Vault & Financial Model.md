# CIRCLE

## Vault & Financial Model

### Shared money, obligations, records, and financial continuity

---

# 1. Purpose

Money appears across almost every meaningful Circle:

* families contribute;
* friends share costs;
* businesses earn and spend;
* communities collect and allocate resources.

Circle therefore needs a unified financial foundation.

But:

> **Circle is not automatically a bank.**

The Vault should begin as a contextual financial layer that can record, coordinate, reconcile, and where legally and technically supported interact with external financial providers.

---

# 2. The Financial Principle

The system must always distinguish:

> **recording money**

from:

> **moving money**

and:

> **holding money**.

These are three different capabilities.

---

# 3. Three Financial Levels

### Level 1 — Record

Circle records:

> “Amina paid Daniel 50,000.”

### Level 2 — Initiate

Circle connects to a provider and initiates:

> “Send 50,000 to Daniel.”

### Level 3 — Custody

Circle actually holds funds.

Custody requires substantially different regulatory, security, and operational architecture.

The initial product should not assume Level 3.

---

# 4. Vault

Vault is the Circle's financial context.

```text id="v4m8k2"
CIRCLE
   ↓
VAULT
   ↓
FINANCIAL OBJECTS
```

The Vault may contain:

* contributions;
* payments;
* expenses;
* obligations;
* invoices;
* settlements;
* budgets;
* balances;
* financial Story.

---

# 5. Financial Objects

Core objects:

```text id="c7h5a1"
TRANSACTION
PAYMENT
CONTRIBUTION
EXPENSE
OBLIGATION
INVOICE
SETTLEMENT
BALANCE
BUDGET
WALLET / ACCOUNT REFERENCE
```

Not every Circle needs all of them.

---

# 6. Transaction

A Transaction records a movement or financial event.

Minimum conceptual fields:

```text id="7r5q20"
FROM
TO
AMOUNT
CURRENCY
DATE
STATUS
PURPOSE
REFERENCE
EVIDENCE
SOURCE
```

---

# 7. Transaction Status

```text id="x6j4x9"
DRAFT
INITIATED
PENDING
CONFIRMED
FAILED
REVERSED
DISPUTED
CANCELLED
```

The system must not describe a pending transaction as completed.

---

# 8. Contribution

A Contribution represents money or value given toward a shared purpose.

Examples:

* family contribution;
* fellowship trip contribution;
* community levy;
* business capital contribution.

It can be:

* monetary;
* in-kind;
* recurring;
* one-time.

---

# 9. Obligation

An Obligation represents money or value that someone owes.

Examples:

* debt;
* IOU;
* unpaid invoice;
* promised contribution;
* reimbursement.

The system should distinguish:

> **owed**

from:

> **paid**.

---

# 10. Invoice

An Invoice represents a request for payment.

It can contain:

* issuer;
* recipient;
* items;
* amount;
* currency;
* due date;
* status;
* evidence.

Statuses:

```text id="v9z5c2"
DRAFT
ISSUED
VIEWED
PARTIALLY_PAID
PAID
OVERDUE
DISPUTED
CANCELLED
```

---

# 11. Expense

An Expense records money spent for a Circle, Project, Asset, or other context.

It should connect to:

* payer;
* beneficiary;
* category;
* project;
* evidence;
* reimbursement status.

---

# 12. Settlement

Settlement resolves financial obligations.

Example:

```text id="p5j1x0"
Expense
→ 40,000

Amina owes Daniel
→ 40,000

Settlement
→ Paid

Obligation
→ Completed
```

This gives Circle a coherent financial history.

---

# 13. Shared Financial Context

Money should remain connected to what it represents.

Bad:

> 250,000 received.

Better:

> 250,000 received for the Community Water Project.

Best:

> 250,000 contribution from three members toward the Community Water Project, with the related receipts attached.

Context is the differentiator.

---

# 14. Family Vault

Typical uses:

* shared household expenses;
* family contributions;
* education funds;
* care expenses;
* property-related records;
* succession-related financial history.

Family Vault should support flexible roles.

---

# 15. Fellowship Vault

Typical uses:

* trip funds;
* shared accommodation;
* group purchases;
* IOUs;
* reimbursements;
* event costs.

The Fellowship Vault should remain lightweight.

It should not feel like accounting software.

---

# 16. Business Vault

Typical uses:

* invoices;
* expenses;
* payments;
* payroll-like records where legally appropriate;
* settlements;
* project finances;
* supplier payments;
* customer payments.

Business Vault can become more sophisticated through capabilities.

---

# 17. Community Vault

Typical uses:

* member contributions;
* treasury;
* project funding;
* disbursements;
* financial reporting;
* contribution history.

Community finance requires stronger governance and auditability.

---

# 18. Shared Money vs Personal Money

Circle must distinguish:

### Personal funds

Owned by an individual.

### Shared funds

Controlled by multiple participants under an agreed arrangement.

### Circle funds

Associated with the Circle.

### External funds

Held by an external financial provider.

This distinction must remain visible.

---

# 19. Non-Custodial Principle

When Circle does not hold money:

> it must never imply that it does.

For example:

**Correct**

> “Balance reported by your linked provider.”

**Incorrect**

> “Your Circle balance.”

unless Circle actually controls those funds.

---

# 20. External Financial Providers

A provider integration may expose:

* account balance;
* transaction status;
* payment confirmation;
* transfer initiation.

Circle should store only what is necessary and permitted.

The external provider remains authoritative for its own financial ledger.

---

# 21. Reconciliation

Circle should help identify differences between:

* recorded transaction;
* provider transaction;
* expected balance.

Example:

```text id="t6h8g4"
Circle record:
100,000 paid

Provider:
100,000 paid

Status:
RECONCILED
```

If different:

> **Needs reconciliation**

rather than silently changing the Circle record.

---

# 22. Financial Evidence

Evidence can include:

* receipt;
* invoice;
* transfer confirmation;
* contract;
* photo;
* provider reference.

Evidence should remain attached to the relevant financial object.

---

# 23. Financial Story

Important financial events can become Story.

Examples:

> “Community raised funds for the water project.”

> “Family completed house construction fund.”

> “Business completed first major contract.”

Financial Story should preserve meaning without exposing sensitive amounts to unauthorized people.

---

# 24. Financial Privacy

Financial visibility should be granular.

Possible visibility:

```text id="m3p8a1"
PARTIES ONLY
AUTHORIZED FINANCE ROLE
CIRCLE MEMBERS
SELECTED MEMBERS
PUBLIC SUMMARY
```

Public financial information should normally expose summaries rather than sensitive underlying details.

---

# 25. Approval

Certain financial actions should require approval.

Example:

```text id="j0q7m2"
Request
 ↓
Review
 ↓
Approve
 ↓
Initiate
 ↓
Confirm
```

The exact flow depends on Circle configuration.

---

# 26. Financial Roles

Possible roles:

### Family

* contributor;
* guardian;
* finance steward.

### Fellowship

* organizer;
* treasurer.

### Business

* owner;
* finance admin;
* manager;
* staff.

### Community

* treasurer;
* steward;
* auditor.

These are contextual roles, not universal permissions.

---

# 27. Limits

Circles may define:

* transaction limits;
* approval thresholds;
* daily limits;
* contribution limits;
* spending categories.

Higher-risk actions can require multiple approvals.

---

# 28. Currency

The system must support multiple currencies.

Every monetary value should store:

* amount;
* currency;
* precision;
* timestamp.

Do not silently convert historical amounts.

Conversions should retain:

* source currency;
* destination currency;
* exchange rate;
* timestamp;
* source.

---

# 29. Mobile Money

Circle's African context makes mobile-money interoperability particularly important.

The architecture should treat mobile money as a provider category rather than creating a separate financial model.

Conceptually:

```text id="d2j6q5"
Circle Vault
    ↓
Payment Adapter
    ↓
Mobile Money Provider
```

The financial domain remains unified.

---

# 30. Cash

Cash must remain a legitimate record type.

Example:

> 25,000 cash contribution.

It should be possible to mark:

* self-declared;
* witnessed;
* evidenced;
* reconciled.

Circle must not pretend cash has provider-level confirmation when it does not.

---

# 31. In-Kind Contributions

Value is not always money.

A Community project may receive:

* cement;
* labor;
* transport;
* equipment.

The system should support in-kind contribution records.

Monetary valuation can be optional.

---

# 32. Financial Disputes

A disputed financial object should preserve:

* original record;
* disputed party;
* reason;
* evidence;
* counterclaim;
* resolution;
* resolution date.

Do not rewrite history to hide disagreement.

---

# 33. Financial Audit Trail

High-impact actions must be auditable:

* creation;
* edits;
* approval;
* payment initiation;
* payment confirmation;
* reversal;
* dispute;
* deletion/retention action.

---

# 34. Financial Lifecycle

```text id="8v7r2k"
CREATED
 ↓
REVIEWED
 ↓
APPROVED
 ↓
INITIATED
 ↓
CONFIRMED
 ↓
RECONCILED
 ↓
STORY / HISTORY
```

Not every transaction follows every state.

---

# 35. Financial Intelligence

Circle may eventually identify:

* recurring obligations;
* unusual changes;
* missed contributions;
* upcoming financial commitments;
* project budget risk.

But financial intelligence should remain explanatory.

Example:

> “Three expected contributions have not yet been recorded.”

rather than:

> “Your finances are unhealthy.”

---

# 36. Financial Possibilities

The Vault can feed the Possibility system.

Example:

```text id="6q5k8j"
Recurring expense
+
Available supplier relationship
+
Business capability
=
Possible cost-saving project
```

But suggestions must not become financial advice without appropriate safeguards.

---

# 37. Monetization Relationship

Vault may be one of the areas where advanced capabilities can become paid.

Potential paid capabilities:

* advanced reconciliation;
* business finance workflows;
* larger transaction volumes;
* advanced reporting;
* integrations;
* governance controls.

Basic shared financial records should remain accessible where possible.

---

# 38. Financial Architecture Principle

The central financial model is:

> **Record → Contextualize → Confirm → Reconcile → Preserve**

not:

> **Bank → Social network → Accounting software**

Circle remains a relationship and continuity platform.

---

# 39. Final Model

```text id="2f0w8h"
PERSON / CIRCLE
      ↓
   FINANCIAL CONTEXT
      ↓
┌─────┼─────────┐
│     │         │
PAYMENT EXPENSE CONTRIBUTION
│     │         │
└─────┼─────────┘
      ↓
 OBLIGATION / SETTLEMENT
      ↓
 EVIDENCE
      ↓
 RECONCILIATION
      ↓
 STORY
      ↓
 NEXT / POSSIBILITY
```

The Vault is therefore not an isolated finance module.

It is financial continuity inside Circle.
