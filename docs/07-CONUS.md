# CONUS — Tenant Specification

**Status:** PROPOSED (D-14: sequencing needs founder sign-off).
**Supersedes:** 13 documents / 31,027 lines in `apps/conus/` (Constitution, Product Strategy, PRD, Zero
Blueprint, Business & Economic Model, Operating Bible, Measurement, Engineering Backlog, Technical
Architecture, QA, UXUI, INFORMATION ARCHITECTURE, weekly plan).
**Rule:** CONUS is an **application on Circle primitives**, not a sibling platform. It does not build a
second identity system.

---

## 1. Product in one line

> **CONUS turns entertainment into participation, participation into cultural signals, and cultural
> signals into recognition and opportunity.**

It is not a content library, not a social network, not a quiz app, not an ad platform. It is the loop that
connects those behaviours:

```text
DISCOVER → EXPERIENCE → PARTICIPATE → SIGNAL → RECOGNIZE → CREATE OPPORTUNITY → RETURN
```

**Sponsor-first.** CONUS is not fundamentally a subscription business; subscriptions are an enhancement
layer, never a wall around the core experience. Nobody should pay merely to remove ads — the paid tier's
answer to "why would someone pay?" must be deeper participation, access, recognition and opportunity.

## 2. The one engine that ships

CONUS Zero previously specified **eight engines at once** — Daily, Short, Founders, Pitch, Challenge,
Live, Fan, Local — with 30 daily topics, a mini-series, 20–50 founder candidates, 10–20 creator pitches,
a major challenge and a live event, on a "lean" team. That is the library, not the loop, and it violated
CONUS's own discipline: *"Spend to prove behavior"* before building native apps, studios, complex AI,
expensive events or large reward pools.

**Ship Daily CONUS. Nothing else until it returns.**

```text
Morning    topic appears
Afternoon  content + interaction
Evening    quiz / challenge
Night      results + ranking + reward + tomorrow's teaser
```

The shape, in full:

> *"How well do you know Cameroonian music?"* → short video, short article, poll, five-question quiz →
> **8/10** → **#14 in Kumba** → **#102 in Cameroon** → *"You're 12 points away from the top 10."*

Every element of that sentence does work: a question, a result, a **local** rank, a **national** rank, and
a reason to come back tomorrow. Deferred to a later season: Short, Founders, Pitch, Challenge, Live, Fan,
Local.

## 3. Audience, deliberately small

CONUS Zero is a **controlled proof**, not a launch. The objective is not maximum users; it is whether
people return, participate, compete, share, recognise winners and create demand.

| Cohort | Size | Purpose |
|---|---|---|
| Participants | **500–2,000** | the behaviour test |
| **Founding Fans** | 100–300 | the first community; their job is to **use it heavily and tell us what is wrong**, not to praise |
| Creators / talents | 10–20 | mixed emerging + established. The question: *can CONUS create opportunity for someone who isn't already famous?* |
| Commercial partners | 3–5 | learn *what a business actually wants from CONUS* before selling packages |
| Founding sponsor | 1–2 | part of the launch story, not an ad buyer |

**Nobody arrives at "Coming soon."** Seed content exists before invitations go out — the initial content
is the seed forest the platform grows from. Quality over arbitrary volume: *one strong idea expressed many
ways* beats many weak ideas produced to fill a calendar.

## 4. The seam — what CONUS must borrow, never rebuild

The previous corpora prove the risk: **all 13 CONUS documents never mentioned Circle once.** An
independent identity system gets built by default, and then the argument for one company dies (D-14).

| CONUS needs | Comes from |
|---|---|
| who a person is, login, session | Circle `Person` / `Account` / `auth-service` (mounted) |
| audience membership, roles | Circle `Membership` / `Role` |
| a participation event becoming durable history | Circle `Story` — CONUS writes, Circle retains |
| reward/points standing | CONUS-owned reputation object, **explicitly not** Circle trust (`05-TRUST.md` §6) |
| payments, mobile money, invoices | Circle `payments` + `billing` (billing: 0 files today — `08-STATUS.md`) |
| notifications, realtime, search | `notification-service`, `realtime-service`, `search-service` (mounted) |
| locales + RTL | `pkg/i18n` (built, unimported) + D-04's four locales |
| per-country rules | `pkg/compliance` (built, unimported) |

**Until Circle primitives exist as code, CONUS runs its own minimal account.** The seam is defined now so
the later migration is mechanical: CONUS stores `circle_person_id` as a nullable external key, and every
CONUS event carries the `02-DOMAIN.md` §9 property set. Migration then becomes a backfill, not a rewrite.

## 5. Economics — and the doctrine wall

CONUS revenue order: **sponsorship → branded experiences → events → selected subscription experiments.**
Do not build complex ad inventory prematurely; do not build the marketplace before there is something
worth competing for.

```text
Revenue − production − reward − distribution − operations − payment costs − creator compensation
       = Contribution Margin          (per campaign, challenge, series, event, sponsor, property)
```

Price anchors, to be validated rather than assumed: **Daily 100 FCFA · Pro 1,000 FCFA/month** (≈ $1.65).
These are the only tested price points in this company and `04-ECONOMICS.md` §5 uses them as the market anchor.

**The doctrine wall (resolves D-14):** Circle forbids advertising and the sale of personal data. CONUS
lists advertising as a revenue stream and behavioural data as an economic asset. These do not reconcile
into one company policy and must not be blended:

- CONUS may sell **sponsorship and branded experiences**. It may not place ads inside Circle surfaces.
- CONUS behavioural data stays in CONUS. **It never enters the Circle trust graph, and Circle data never
  feeds a CONUS ad product.**
- Recognition must be **earned, not purchased**. "A creator can become a founder; a founder can become a
  partner" is a ladder, **not a payment tier** — and it must never be conflated with the Founding Circle
  charter in `04-ECONOMICS.md` §7.
- Any commercial use of audience data must be consented and lawful. **CONUS's data is not the company's
  asset to spend.**

## 6. Product rules worth keeping, stated once

1. **The viral unit is a URL.** Share a quiz, a result, a creator, a challenge, a ranking, a story.
   The recipient lands **directly on the experience** — not on "download our app."
2. **Mobile-first and low-bandwidth is a product constraint, not an optimisation for later.** A PWA is the
   right first surface. The test: *can someone discover CONUS from a shared link and participate immediately?*
3. **Every meaningful action emits measurable data.**
4. **Human judgement stays in the system.** Algorithms calculate; humans decide.
5. **Operate for the editorial team.** If publishing a topic or awarding recognition needs an engineer
   every time, the product is not operationally ready.
6. **Language is direct:** Play · Watch · Vote · Enter · Follow · Win · Discover · Share · Rise.
   Never "engage with our innovative ecosystem" or "unlock synergistic experiences."
7. **It should not feel:** corporate, bloated, confusing, advertising-heavy, gameified for its own sake,
   or like a dashboard.
8. **Daily Question:** *why should someone come back today?* If CONUS cannot answer it, the habit engine
   is not working — that is the failure signal, not low traffic.
9. **Time horizons:** days for participation, weeks for momentum, seasons for entertainment, years for
   careers, decades for cultural memory.
10. **Domain boundaries before distributed infrastructure.** CONUS is a modular monolith mounted in the
    existing hosts — not eight services, and not a second platform stack.



## 7. Measurement

**North Star: Weekly Qualified Participants (WQP)** — distinct people performing ≥ 1 qualifying
meaningful participation action in a rolling 7-day window (completed quiz, valid poll response, challenge
entry, competition vote). Exclude test accounts, internal staff, confirmed fraud, invalid/duplicate events.
Count anonymous participation separately; **never silently mix it with authenticated WQP.**

WQP is stronger than registrations, visitors, page views or followers because **a person counts because
they did something.** View it always alongside: daily return rate, participation completion rate, share
rate, follow rate, local participation, creator engagement, reward effectiveness, sponsor engagement, and
**cost per meaningful attention** — the sponsor-facing unit.

CONUS metrics never become Circle metrics. Circle's North Star stays MCT per 1,000 people
(`06-MEASUREMENT.md` §2).

## 8. What was cut, and the two lessons kept

Cut: a 4,066-line Technical Architecture, a 2,922-line Engineering Backlog, a 2,764-line PRD, a
2,711-line QA spec, a 2,673-line Operating Bible (14 engines), 2,608 lines of Business Model (78
sections, 9 revenue streams), 2,525 lines of Product Strategy, 2,391 lines of IA, 2,390 lines of UXUI,
2,192 lines of Measurement, 1,917 lines of Zero Blueprint — **all written before CONUS had a single user.**

Kept as permanent lessons, because they were correct:
- **The loop, not the inventory.** One idea, many expressions.
- **Spend to prove behaviour.** The discipline the document set itself violated is now the gate on §2:
  Daily only, because Daily is the cheapest way to prove return behaviour exists before funding anything else.
