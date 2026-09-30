# ADR-0009: Locale Set and RTL Requirement

**Status:** Accepted
**Date:** 2026-09-30
**Deciders:** Docs sync (docs follow the shipped code)
**Supersedes:** `Language & Localization.md` §2 ("English + French")

## Context

Three different answers existed:

| Source | Locales |
|---|---|
| `Language & Localization.md:26-32` (§2) | **English + French** as first-class; Cameroon + international context |
| `doc/africa_os_i18n_geo_compliance_spec.md` | builds out **Arabic, Hausa, Amharic, Swahili, Portuguese** (mention counts: Arabic 13, Hausa 10, Amharic 5) |
| **Shipped code** `combi/apps/cast/i18n/routing.ts:6-11` | `en-NG` (Nigerian English), `pcm-NG` (Nigerian Pidgin/Naija), `fr-CM` (Cameroonian French), `ha-NG-Arab` (**Hausa Ajami, RTL**), `defaultLocale: en-NG`, `localePrefix: "always"` |

The docs also under-specify what the code already does: Pidgin appears at §15
(193-195) as "a community voice and language option where appropriate", and the
Accessibility section (§32, lines 400-408) lists typography, screen readers and
text-expansion space but contains **no RTL requirement at all** — despite an RTL
locale being live.

## Decision

**The Circle V1 locale set is the one already shipped in Cast:**

```text
en-NG      Nigerian English        first-class, default
fr-CM      Cameroonian French      first-class, equal standing (Language §4)
pcm-NG     Nigerian Pidgin         first-class (NOT an "option where appropriate")
ha-NG-Arab Hausa (Ajami script)    first-class, RTL
```

**RTL becomes a written, non-negotiable requirement.** Because `ha-NG-Arab` is
live, every layout, icon, carousel, table and notification surface must be
direction-aware. Added to `Language & Localization.md` as a dedicated section
that the Accessibility section must reference.

Additional decisions carried from the existing docs (unchanged, only now
consistent with code): fallback chain `USER → CIRCLE → ORIGINAL CONTENT → SYSTEM
DEFAULT` (§35); translation must not bypass permissions (§33, kept verbatim —
this is gold); `PRD` §101 MVP line updated from "English/French foundation" to
the four-locale set; `doc/`'s broader Africa-wide registry (Amharic, Swahili,
Portuguese, Arabic) is treated as **platform capability, not Circle scope** — it
describes what the i18n layer *can* do, not what Circle commits to launch.

## Consequences

- Positive: docs stop contradicting running code; the code was ahead.
- Positive: elevating Pidgin to first-class matches `Product Philosophy` §18
  ("culturally grounded, not culturally locked") and the community-language
  intent already in `PRD` §54.
- Negative: RTL + four locales roughly triples UI QA surface versus English-only,
  and text-expansion space must be designed for from the start.
- Negative: `Language & Localization.md` §2's simple "English + French" framing
  must be rewritten, and `PRD` §101's MVP bullet too.
- Risk: `ha-NG-Arab` implies Arabic-script font and shaping requirements that no
  Circle document currently addresses; flagged as an open item in the readiness
  ledger rather than silently assumed.

## Alternatives Considered

- **Docs win (EN+FR only)** — rejected: the code already ships Pidgin and RTL
  Hausa; pretending otherwise hides a live requirement.
- **Adopt the full `doc/` registry (7+ locales)** — rejected for launch: no
  content, review or support capacity behind it; it is the *platform's*
  extensibility claim, not a Circle commitment.
- **Pidgin as a "community voice option" only** — rejected: inconsistent with a
  shipped locale and with the stated cultural positioning.
