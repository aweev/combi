# CIRCLE

## Language & Localization

### Building a system that speaks to people, not merely translating an interface

---

# 1. Purpose

Language is not simply a UI translation problem.

Circle's language architecture must support:

* multilingual content;
* localized interface text;
* different cultural contexts;
* different expressions of identity;
* local terminology;
* community voice.

---

# 2. Core Language Decision (ADR-0009)

The V1 locale set is the one **already shipped** in `apps/cast/i18n/routing.ts` —
the documents previously claimed "English + French" while four locales, including
an RTL one, were live in code.

```text
en-NG       Nigerian English        first-class · default
fr-CM       Cameroonian French      first-class, equal standing (§4)
pcm-NG      Nigerian Pidgin         first-class (NOT an optional "community voice")
ha-NG-Arab  Hausa in Ajami script   first-class · RTL
```

This reflects the intended broad Cameroonian and Nigerian context.

The wider registry described in `doc/africa_os_i18n_geo_compliance_spec.md`
(Amharic, Swahili, Portuguese, Arabic) is **platform capability, not Circle
scope**: it states what the i18n layer can do, not what Circle commits to launch.

---

# 3. English

English should be treated as a first-class language throughout:

* interface;
* system messages;
* documentation;
* help;
* notifications;
* transactional communication.

---

# 4. French

French should have equivalent first-class support.

French should not be treated as:

> English translated after the product is built.

The content architecture must support both from the beginning.

---

# 5. Content Model

Translatable content should be separated from the underlying object.

Example:

```text
STORY
│
├── English title
├── French title
├── English description
└── French description
```

The Story itself remains one object.

---

# 6. Translation Architecture

Circle should distinguish:

```text
SOURCE CONTENT
TRANSLATION
LOCALE
LANGUAGE
```

This prevents duplicated objects.

---

# 7. User Language

Each user should have:

* preferred interface language;
* optional content language preferences;
* fallback language.

---

# 8. Circle Language

A Circle may establish preferred languages.

Example:

> Business → English + French

or:

> Community → French.

---

# 9. Content Language

Every significant user-created content object should optionally have a language.

Examples:

* Story;
* Announcement;
* Document metadata;
* Event;
* Description.

---

# 10. Multiple Languages in One Circle

A Circle may contain content in several languages.

The system should not force all members to use one language.

---

# 11. Translation Availability

When content is available in another language:

> Show translated version.

When not:

> Show original.

Optionally:

> Translate.

---

# 12. Translation Status

Translations may have states:

```text
NOT TRANSLATED
MACHINE TRANSLATED
HUMAN TRANSLATED
REVIEWED
OUTDATED
```

---

# 13. Translation Provenance

Circle should know:

> who translated something;

> when;

> from which source;

> whether the source changed afterward.

---

# 14. Machine Translation

Machine translation can accelerate multilingual participation.

However, automatically translated content should not silently be treated as human-reviewed.

---

# 15. Cameroonian Pidgin / Mboko

Cameroonian Pidgin is a **first-class locale** (`pcm-NG`), not an option offered
"where appropriate" — it ships in `apps/cast/i18n/routing.ts` and carries the
cultural positioning in `Product Philosophy.md` §18 (ADR-0009).

The product should distinguish:

> formal interface language

from:

> community expression.

---

# 16. Camfranglais

Camfranglais may similarly function as an expressive / community language layer.

The architecture should permit it without forcing every system string to be translated into it.

---

# 17. Voice vs Interface Language

This distinction is important.

Example:

The interface may be:

> English

while a user writes a Story in:

> Camfranglais.

Both should be valid.

---

# 18. Language Detection

Circle may detect likely language for convenience.

But detection should not silently change important user preferences.

---

# 19. Local Terminology

Certain terms may need localization beyond direct translation.

Example:

> Family

may require different culturally natural expressions depending on context.

Localization should prioritize:

> meaning

over:

> literal word replacement.

---

# 20. Dates

Circle should support localized:

* date formats;
* time formats;
* weekdays;
* months.

---

# 21. Currency

Currency should be contextual.

A Circle may use:

* XAF;
* EUR;
* USD;
* other supported currencies.

Currency should not be hardcoded into archetypes.

---

# 22. Numbers

Number formatting must support locale conventions.

---

# 23. Time Zones

Events and commitments must store time correctly.

The system should distinguish:

> absolute time

from:

> displayed local time.

---

# 24. Address and Location

Location fields should be flexible enough to support:

* countries;
* regions;
* cities;
* neighborhoods;
* informal place descriptions.

---

# 25. Locality Without Overfitting

Circle should support Cameroon deeply without making the underlying architecture impossible to use elsewhere.

The principle is:

> **Cameroon-first, globally extensible.**

---

# 26. Cultural Context

The product should permit culturally meaningful structures without forcing every user into them.

Examples:

* extended family;
* community groups;
* fellowships;
* associations;
* informal businesses.

---

# 27. Naming

Users should be able to name Circles naturally.

A Circle could be:

> "The Nfor Family"

or:

> "Molyko Creatives"

or:

> "ABC Engineering Ltd."

The archetype remains separate from the user's chosen name.

---

# 28. Language and Possibility

Possibility Expander should operate across languages.

A French-speaking user should not receive a weaker experience than an English-speaking user.

---

# 29. Search Across Languages

Search should eventually support cross-language understanding.

A French query could find relevant English content when the user has permission to access it.

---

# 30. Language and AI

AI language processing should preserve:

* names;
* local expressions;
* cultural context;
* intended meaning.

It should avoid incorrectly "correcting" local language into formal English or French.

---

# 31. Voice

Future voice interfaces should support multiple languages where technically feasible.

The underlying objects remain language-neutral.

---

# 32. Accessibility

Localization should include:

* readable typography;
* screen-reader compatibility;
* sufficient text expansion space;
* accessible language selection;
* clear translated labels;
* **right-to-left layouts** (see §32a — mandatory, not optional).

---

# 32a. Right-to-Left Requirement (ADR-0009)

`ha-NG-Arab` ships Ajami-script Hausa, so RTL is a live requirement, not a future
one. Every surface must be direction-aware:

* mirrored navigation, reading order, carousels, tables and date pickers;
* flow-relative spacing (start/end, never left/right);
* icon mirroring where direction carries meaning;
* mixed-direction content (Latin names or amounts inside Arabic-script text);
* Arabic-script font shaping and ligature rendering;
* `dir` set per locale at the document root;
* notification and deep-link surfaces included.

This requirement did not appear anywhere in the earlier version of this document
despite the locale already existing in code.

---

# 33. Translation and Permissions

Translation should not bypass permissions.

If a user cannot access the original object:

> they cannot access its translation.

---

# 34. Translation and Versioning

When source content changes:

> translations may become outdated.

Circle should indicate this where necessary.

---

# 35. Language Fallback

Recommended fallback:

```text
USER LANGUAGE
      ↓
CIRCLE LANGUAGE
      ↓
ORIGINAL CONTENT
      ↓
SYSTEM DEFAULT
```

The exact fallback can depend on context.

---

# 36. Localization Governance

A central language system should manage:

* translation keys;
* terminology;
* language packs;
* pluralization;
* formatting;
* translation status.

---

# 37. No Hardcoded Text

Product developers should avoid hardcoding user-facing strings directly into application logic.

All interface copy should be localizable.

---

# 38. Language Principle

> **Language should be a property of the experience, not a fork of the product.**

---

# 39. Final Principle

Circle should feel like:

> **your Circle in your language**

rather than:

> an English product translated into another language.
