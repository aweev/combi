# Decision Register

**Status:** LIVING — the single place a decision is recorded. Documents link here; they never restate.
**Supersedes:** 11 ADR files in `combi/docs/adr/` + 10 in `api/docs/adr/` (which collided — two files
numbered 0002, `0002-event-bus-redis-streams.md` and `0002-platform-tech-stack.md`).

---

## How to read the statuses

| Status | Meaning |
|---|---|
| **DECIDED** | Settled. Contradicting text has been rewritten or deleted. |
| **DECIDED BY CODE** | The code already chose; this register records it and deletes the losing docs. |
| **PROPOSED — money** | Changes price, billing or brand. **Recommendation only. Not law. Nothing may be built on it as though decided.** |
| **PROPOSED — founder** | Affects the flagship pitch or the founder's stated intent. Requires sign-off. |
| **OPEN** | Unknown. Stated as a blank, never filled with a guess. |

**Governance rule replacing the old `Status: AGREED`:** internal consistency is not build readiness.
No document may claim a system is "safe to build from." Only `08-STATUS.md` may state what exists in
code. This exists because the old index marked **24 of 26** documents `AGREED` — defined as "internally
consistent; safe to build from" — while four of the decisions they depended on were unsigned and the
flagship had zero code.

---

## Settled

### D-01 · Archetypes — DECIDED
Exactly four: `FAMILY`, `FELLOWSHIP`, `BUSINESS`, `COMMUNITY`.
A Person is **not** a fifth archetype: a Circle is defined as a *collective* context, so one person
cannot be one. The individual surface is served by `Person` + `Personal Face`.
*Retired terms:* `Individual`, `Me` (as a model term; survives as a nav label only), `Personal` (as an
archetype), `PERSON` (in the archetype list).
**Consequence:** a private single-member Circle is a Circle with one member, not a new archetype.

### D-02 · Face taxonomy — DECIDED
Exactly three Face types: `PERSONAL`, `BUSINESS`, `PSEUDONYMOUS`. `ROLE-DELEGATED` is deferred, not added.
**Public / Private are visibility levels, never Face types.** Making "public" an identity instead of a
disclosure setting would break contextual privacy. "Private Space" is a *mode* of a Personal Face
(person-owned Story/Next belonging to no Circle) — the concept survives, its taxonomy is demoted.

### D-03 · Nesting — DECIDED
Two mechanisms only:
- **Branch** — parent → child, **depth capped at 1**. The child is a Circle in its own right and cannot branch further.
- **Affiliation** — peer link between two Circles, each side explicitly choosing what it discloses.

**Unbounded recursion is not supported,** and the phrase "intentional audience layer" is deleted — it
named no object in the domain model. A broadcast/audience need must arrive as a capability through the
Expansion Rule, never as a third nesting shape.
*Why it matters:* depth 1 makes effective-permission resolution touch at most one parent, keeping
permission inheritance, contextual privacy and lawful deletion tractable. Deep graphs make all three a
compliance hazard. Genuinely deep organisations (association → region → chapter → team) are handled by
Affiliation chains plus a Directory capability if demand ever appears.

### D-04 · Locale set — DECIDED (docs follow shipped code)
```text
en-NG       Nigerian English      default
fr-CM       Cameroonian French
pcm-NG      Nigerian Pidgin       first-class, NOT an "option where appropriate"
ha-NG-Arab  Hausa in Ajami script  RTL — mandatory layout requirement
```
Live in `apps/cast/i18n/routing.ts`; the old locale doc's "English + French" was behind the code.
**RTL is non-negotiable:** every layout, icon, carousel, table and notification surface must be
direction-aware, because an RTL locale already ships.
*Scope note:* the broader registry in the archived `doc/` specs (Amharic, Swahili, Portuguese, Arabic) is
**platform capability, not a Circle commitment** — it describes what the layer can do, not what we launch.
**Cost acknowledged:** four locales + RTL roughly triples UI QA surface versus English-only, and
Ajami script shaping/font requirements are **not yet addressed anywhere** — tracked as D-16.

### D-05 · Vocabulary budget — DECIDED
Seven user-facing nouns, forever: **Circle · Face · Story · Now · Next · Vault · Possibility.**
Everything else is internal: Activity, Commitment, Event, Project, Object, Document, Media, Transaction,
Relationship, Role, Permission, Evidence, Place, Time, Money, Capability, and all engine internals.
**Banned as synonyms** (they describe products we refuse to be): feed, timeline, posts, journal,
dashboard, inbox, goals, roadmap, backlog, wallet, bank, followers, likes, suggestions.
Calling Story a "feed" is a defect, not a stylistic choice. A new user-facing noun requires editing this line.

### D-06 · State machines — DECIDED
One subscription machine, one account machine, one data machine, one membership machine.
**No lifecycle may secretly control another.** Definitions live in `02-DOMAIN.md` §3 and nowhere else.
Naming fixed: `GRACE` (not `GRACE_PERIOD`); `RESTRICTED` in code with "Limited Mode" as the user-facing
label only; `CLOSED` (replacing `TERMINATED`); `CANCELLED`/`EXPIRED` are **causes**, not states;
`RETRY` is a mechanism inside `PAST_DUE`, not a state; `DELETION_PENDING`/`DELETED` are **data** states
and must never appear in the subscription enum.

### D-07 · Event broker — DECIDED BY CODE
**NATS JetStream is the event bus.** The host proves it: `pkg/fxcore/module.go:201`
`ProvideEventBus(cfg, nc *nats.Conn, …)` — a Redis bus cannot be mounted without changing the host.
The five conflicting claims (api ADR-0002 "Redis Streams, Accepted"; `plan.md` "keep Redis Streams";
`task.md` "NATS JetStream"; `pkg/events/README.md` "Transport: Redis Streams"; `doc/` "Redpanda/Kafka")
resolve as:
- `pkg/events/redis/` (bus, reaper, tests) — **dead code on the bus path.** Delete unless `pkg/queue` needs it (D-08).
- `pkg/events/README.md` — rewritten to name NATS. It also carried 7 stale "NGO Platform" strings.
- Redpanda/Kafka — **not adopted.** An unimplemented `doc/` spec; a third broker is a new dependency for no shipped benefit.
- `services/platform/event-bus-service/` — 1-file skeleton, deleted (duplicates `pkg/events`).
**ADR numbering is de-duplicated by this file:** exactly one decision register exists, in `combi/docs/`.

### D-08 · Queue vs. bus vs. Temporal — DECIDED
Three mechanisms, deliberately distinct; the previous docs mixed them:
```text
Event    (pkg/events, NATS)         = a fact that already happened. Fire, fan out, replay.
Job      (pkg/queue, Redis Streams)  = a command to execute, retry, dead-letter.
Workflow (pkg/temporal)             = long-running, multi-step, human-in-the-loop orchestration.
```
`plan.md`'s "replace the queue system with Temporal" is **reversed**: Temporal does not replace a
lightweight job queue, and routing every async task through a workflow engine adds a heavyweight
dependency for work that needs none. `pkg/queue` stays. Redis therefore remains in the stack **as a job
broker** — the correct reading behind the keep/delete confusion in D-07.

### D-09 · Platform topology — DECIDED
**Three host binaries:** `cmd/platform-server` (all mounted services, gRPC :50051),
`cmd/workflow-engine` (Temporal workers; own lifecycle), one binary per shipped app.
**Services are library modules, not processes** — no per-service `main` package. Generate with
`go run ./cmd/service-init -tier platform <name>`.
Consequence: the ~20 legacy per-service `cmd/server/main.go` stubs are **not repaired; they are deleted**
with the services cut in `00-MANIFEST.md` §8. `api-gateway-service` goes too — the signature artifact of
the topology we abandoned.

### D-10 · Dormancy, payment failure and data loss — PROPOSED (money gate)
**Recommended: no data loss and no identifier recycling on payment failure or dormancy.**
1. Free accounts never expire and never lose data. Free access does not lapse for non-payment.
2. A lapsed **paid** subscription degrades to `RESTRICTED` (Limited Mode: view, communicate with
   existing members, export, renew, reactivate). `INACTIVE` then `CLOSED` end the *commercial*
   relationship only.
3. Deletion requires an explicit user request or a legal obligation — never non-payment, never dormancy.
4. **Identifiers are not reclaimed for dormancy.** The waitlist / long-list / reactivation-queue
   mechanism is **deleted.**
5. Dormancy may reduce *expensive* resources — cold storage tiering, background processing — the cost
   saving without the harm.

*Why this is the recommendation:* the product's only differentiator is continuity. Deleting a person's
accumulated obligations and history and then making them queue to return attacks the one thing being
sold. It also removes the incentive to destroy records for revenue reasons, and eliminates a class of
consumer- and data-protection risk (deleting records because someone could not pay).

**Conflict resolved here, which the old corpus left live:** `Lifecycle & Account Status.md` §19–§20 and
line 325 still specified a full WAITLIST flow ("RETURN REQUEST → ELIGIBILITY → WAITLIST → CAPACITY
AVAILABLE → ACTIVATION") *after* the ADR that claimed to delete it — and in the document ranked #7 of 26
in authority, above the #17 document that recorded the deletion. That mechanism is now **deleted from
everywhere** and invariant 5 in `README.md` §3 blocks its return.
**Tension to accept honestly:** indefinite retention fights data-minimisation. Resolution: **anonymise,
do not erase** — shared history survives a person's departure with personal fields removed (see `05-TRUST.md` §5).
**Consequence for pricing:** a dormant-data tail must be priced into storage economics; requires a
hot → cold → archive tiering policy rather than deletion. Deferred storage billing for large
ORGANIZATION accounts is defensible; for free/personal Circles it is not.

### D-11 · Billing unit — PROPOSED (money gate)
**Recommended: `CCU` (Circle Capacity Unit) — a Circle's capacity consumption.**
- **Metered dimensions:** active seats, storage, media/ingest volume, AI/compute usage, transactions
  processed, API calls, connected integrations. These map to real cost lines.
- **Who is billed:** the `BILLING ACCOUNT` that owns a Circle — never a person for merely belonging.
- **Never billable: the identifier namespace.** Scarcity of account numbers is a technical capacity
  concern and must not leak into pricing.
- **Price floor:** personal context, `FAMILY` and `FELLOWSHIP` sit at **zero price** with a generous
  fixed allowance. `BUSINESS` and large `COMMUNITY` are the metered segments.
- **CCU is never customer-facing.** Customers see a plain-language capacity label (D-16 item 9).

*Why one unit matters:* the corpus previously defined the commercial unit four mutually exclusive ways
(capacity, "open question", "must be finalised", scarce identifiers). Per-seat, per-Circle-capacity and
scarce-identifier models produce **opposite behaviour for the same customer**; until one wins, price,
capacity bands, the cap and the entire inactivity lifecycle are uncomputable.
*Rejected:* per-person subscription (charges a person for belonging to multiple Circles);
scarce-identifier rationing (monetises a technical limitation); pure usage-based (unpredictable invoices).
**Blocking fact: no CCU metering exists in code.** No price can be invoiced until it does (`08-STATUS.md` §4).

### D-12 · Pricing curve — PROPOSED (money gate)
**Recommended: flat published plan price + metered add-ons + a rate cap. Delete doubling.**
```text
PLAN BASE PRICE     published, predictable, per plan, per currency
+ METERED ADD-ONS    CCU dimensions above the included allowance
+ BUNDLE             where capabilities naturally work together
- RULES              sponsorship, nonprofit, education
= PRICE
```
1. **Rate cap (every plan):** a bill may escalate by **at most one band per billing year**, with notice
   and grandfathering. This replaces "there is a maximum price" theatre with a promise a customer can
   verify against their own invoices.
2. **Absolute cap (ORGANIZATION only):** above it, the relationship moves to custom/enterprise terms.
3. Band escalation, where used, is **linear and capped — never `2^N`.**

*Conflict resolved:* the founder's stated intent was a fee that **doubles per million users** (exponential).
At a $5 base that reaches **$2,560/month at 10M users**, where the documents would have charged $50. It
cannot coexist with a guaranteed cap — either the cap binds immediately (the curve is decorative) or it
does not (the price is absurd) — and on a *global* user count it charges a three-person Family for other
people's growth. It also contradicts two written promises: "your price will never exceed this cap" and
"I understand what Circle costs."
*Rejected:* doubling per milestone (founder intent, overruled on the arithmetic above); linear `N × P`
as a general model (retained for ORGANIZATION only); flat forever (ignores real infrastructure cost of
large Circles).
**Consequence:** abandons a headline "price rises with our success" story that had marketing appeal and
no unit-economics basis.

### D-13 · Brand and product hierarchy — PROPOSED (money gate; rename is breaking)
```text
Combi        = company and monorepo (github.com/combi/*, DB "combi", event prefix "combi:")
Circle       = flagship product; the trust substrate
  └── CONUS  = an application running on Circle primitives
  └── Cast   = an application (or capability), not seven services
  └── tools  = capabilities installed into Circles
NGO          = origin story of the deleted api/ codebase. History only.
Africa OS    = name of the platform-infrastructure specs formerly in doc/ (now archived).
```
Four simultaneous identities existed with **zero cross-references**: all 13 CONUS docs never mention
Circle; `plan.md` had **no service category for Circle at all**, so the flagship could not be placed by
the approved migration plan. Circle gains a home under `services/apps/circle/`.
**Brand items that remain actionable.** The legacy `api/` monolith has since been deleted from the
workspace, so its `config.yml` values (`issuer: ngo-platform.org`, `dbname: ngo_platform_db`,
`totpIssuer`, `no-reply@ngo-platform.org`) are no longer migration targets — they survive only in the
archive at `README.md` §4. **What is still live, in `combi/` code, and user-visible at runtime:**

| Location | String | Risk |
|---|---|---|
| `services/platform/workflow-service/activities.go:294` | `https://sign.ngo-platform.org/contract/…` | **BREAKING if changed** — emitted into signing links |
| `services/platform/organization-service/workers.go:83` | `https://app.ngo-platform.org/join?token=…` | invite links sent to real people; needs the frontend URL (still hard-coded, `08-STATUS.md` §5) |
| `services/platform/notification-service/processor/processor.go:56` | `dev@ngo-platform.org` | sender address; needs a real mailbox, affects deliverability |
| `services/platform/integration-service/adapters/stripe_adapter.go:78` | Stripe `ReturnURL` `ngo-platform.org` | hard-coded, comment says "should come from config" |
| `pkg/config/loader.go:33` | `SetEnvPrefix("NGO")` | **renaming is breaking**: every `NGO_*` env var in deployment would stop resolving. `combi/config.yml` documents these names, so they are contract, not cosmetic |

`combi/config.yml` itself is already clean (`user: combi`, `dbname: combi`, `channelPrefix: combi:events:`,
`issuer: combi`). The stale "NGO Platform" strings in `pkg/events/README.md` were rewritten with the
NATS correction. **Conclusion: the brand problem moved from `api/` to `combi/`** — do not treat deleting
the old tree as having resolved D-13.

*Rejected:* Circle as the company name (code reality is `combi`; renaming code costs more than renaming
marketing); keeping "NGO Intelligence Platform" as a second product (no document references it); four
sibling products with no hierarchy (cannot answer "who reuses whose identity primitives", which is the
entire economic argument for one monorepo).

### D-14 · CONUS ↔ Circle relationship — PROPOSED (founder)
**Sequencing: Circle first as platform, CONUS as its first tenant, Cast behind the contract.**
Launching all four as siblings is precisely what produced four corpora that never referenced each other.
**The load-bearing risk, stated in full:** *Circle must ship the primitives CONUS needs (identity,
membership, events, reward hooks) or CONUS keeps building its own and the argument for one company dies.*
Until Circle has code, CONUS adopts no Circle identity — but the seam is defined now (`07-CONUS.md` §4)
so the migration is mechanical rather than a rewrite.
**Money doctrine conflict, resolved by separation rather than compromise:** Circle forbids advertising and
the sale of personal data; CONUS lists advertising as a revenue stream and behavioural data as an economic
asset. Both survive **only if the doctrines stay in separate products.** They do not merge into one
company-wide policy, and no CONUS ad or data product may touch Circle's trust substrate. See `07-CONUS.md` §5.

### D-15 · Cast — PROPOSED (founder)
**One Cast app, several capabilities.** `services/apps/cast/*` is 7 directories holding **7 Go files
total** — effectively unbuilt — while the only real application code went to `apps/ngo/` (46 files).
Cheapest correct answer: one product surface, capabilities installed through `03-CAPABILITY.md`.

### D-16 · Open — recorded as blanks, never guesses
| # | Question | Status |
|---|---|---|
| 1 | "Active seat" definition — what makes a seat billable in a period? | **OPEN.** Blocks automated invoicing. |
| 2 | CCU **weights** per dimension | **OPEN.** D-11 lists dimensions but assigns no priority. |
| 3 | Currency and regional tiers | **OPEN.** Tax handled in the billing engine, not plan definitions. |
| 4 | Payment providers + fee structure (C5) | **OPEN.** Mobile-money fees ≠ card fees; materially changes margin. |
| 5 | C1–C8 cost inputs | **OPEN.** None measured; gates every published price. |
| 6 | Ajami script shaping / RTL font requirements | **OPEN.** No document addresses it; the locale ships anyway. |
| 7 | Is MCT's "3 Story entries" the right bar? | **OPEN.** Calibrate on the first real cohort. |
| 8 | Which brand faces users — Combi, Circle, or "Combi: Circle"? | **OPEN.** |
| 9 | CCU plain-language display name | **OPEN.** Customers must never see "CCU". |

**Do not resolve these by writing a number.** A false number in a pricing document becomes a promise to a
customer. Resolve with a pilot, a hosting estimate, or a decision — then edit this table.

---

## The five signatures requested

| # | Decision | Gate | What is blocked until signed |
|---|---|---|---|
| 1 | **D-11 CCU** (+ resolve D-16.1 "active seat") | money | pricing model, capacity bands, the cap, all metering work |
| 2 | **D-12 pricing curve** | money | the billing engine, the port of the legacy financial service |
| 3 | **D-10 dormancy / no data loss** | money | the data-retention schedule, the safety policy, D-03's premise |
| 4 | **D-13 brand hierarchy** + Circle's place in the topology | money | naming migration, the DB rename, the dual-accept token window |
| 5 | **D-07 broker** (already decided by code — needs ratification so docs stop contradicting it) | structural | `pkg/events` cleanup, event namespace, one ADR tree |

D-14 (CONUS as tenant) and D-15 (Cast) should be signed in the same sitting; they are cheap and they stop
two product lines from silently forking the identity layer.


