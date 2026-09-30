# ADR-0006: Brand and Product Hierarchy

**Status:** Proposed — **money gate (rename affects tokens, DB and email), requires founder sign-off**
**Date:** 2026-09-30
**Deciders:** Founder (pending)

## Context

The workspace carries four simultaneous identities with no stated relationship:

| Artifact | Identity claimed |
|---|---|
| `combi/apps/circle/*.md` | Circle, flagship product (21 docs, zero mentions of "Combi") |
| `combi/apps/conus/*.md` | CONUS, media/sponsorship product (13 docs, **zero mentions of "Circle"**); `Business & Economic Model.md:7`: "CONUS is not fundamentally a subscription business" |
| `combi/apps/cast/` + `services/apps/cast/` | Cast, 7 services (arena, burnbook, cortex, courtroom, creator-studio, squads, syndicate) |
| `api/` | `api/README.md:1` "NGO Intelligence Platform Platform Backend"; `api/config.yml:23` `ngo_platform_db`, `:77` issuer `ngo-platform.org`, `:87` `NGO Platform` TOTP issuer, `:216` `no-reply@ngo-platform.org` |
| `doc/` | "africa_os_*" service registry and i18n specs |
| `plan.md:40` | Approved structure: `platform/`, `apps/cast/`, `apps/ngo/` — **no home for Circle** |

Module naming is already settled by `plan.md:36` (`github.com/combi/*` ✅) and
`combi/config.yml` (`user: combi`, `dbname: combi`, `channelPrefix:
combi:events:`, `serviceName: combi-platform-server`, `issuer: combi`), so the
`api/` strings are stale legacy rather than a competing decision.

## Decision (recommended)

```text
Combi        = company and monorepo (github.com/combi/*, DB "combi", event prefix "combi:")
Circle       = flagship product; the identity/relationship/capability platform
  └── CONUS  = an application running on Circle primitives
  └── Cast   = an application (or app family) running on Circle primitives
  └── tools  = capabilities installed into Circles (see Capability Contract)
NGO          = the origin story of the legacy api/ codebase; documented as history only
Africa OS    = the platform-infrastructure spec name in doc/; rename to Combi Platform specs
```

`plan.md:40` gains a fourth category, `apps/circle/`, so the migration plan can
actually host the flagship (`services/apps/` currently contains only `cast` and
`ngo`).

## Naming changes — annotated, **not executed**

Two are breaking for live systems and must be migrated deliberately:

| File:line | Current | Target | Risk |
|---|---|---|---|
| `api/config.yml:77` | `issuer: "ngo-platform.org"` | `combi` | **BREAKING** — invalidates every issued token; needs a dual-accept window |
| `api/config.yml:23` | `dbname: "ngo_platform_db"` | `combi` | **BREAKING** — DB rename/migration |
| `api/config.yml:87` | `totpIssuer: "NGO Platform"` | product name | User-visible on authenticator apps; cosmetic but confusing mid-change |
| `api/config.yml:104` | `serviceName: "ngo-platform-server"` | `combi-*` | Telemetry/dashboard labels only |
| `api/config.yml:216` | `no-reply@ngo-platform.org` | brand domain | Requires a real mailbox; affects deliverability and spam |
| `api/README.md:1` | "NGO Intelligence Platform Platform Backend" (doubled word) | accurate title | None |
| `api/README.md` CI badge | slug `the NGO Platform-ev/server` contains spaces — invalid URL | fix or remove | None; badge is currently broken |
| `combi/pkg/README.md` | "`NGO_` env overrides" | verify against `pkg/config` code | Doc-vs-code drift check |

## Consequences

- Positive: one product can be described without four competing names; makes
  the "Circle as hub" thesis expressible as a real dependency graph instead of
  a slogan, and gives CONUS's 0-mentions-of-Circle gap a documented answer.
- Positive: settles whether NGO work is a product line or legacy code.
- Negative: token and DB renames are real migration projects, not edits — hence
  annotated rather than changed in this sync.
- Negative: CONUS docs will need a relationship section before they can cite
  Circle; `Platform Relationship.md` (authored in this sync) is that bridge.

## Alternatives Considered

- **Circle as company name** — viable, but `github.com/combi/*` and the `combi`
  DB are already the code reality; renaming code is more expensive than renaming
  marketing.
- **NGO Intelligence Platform retained as a second product** — rejected: no
  Circle or CONUS document references it; it appears to be the original framing
  of `api/`, not a live product decision.
- **Flat brand with no hierarchy (all four as sibling products)** — rejected:
  cannot answer "who reuses whose identity/relationship primitives", which is
  the entire economic argument for one monorepo.
