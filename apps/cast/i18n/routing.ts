// apps/cast/i18n/routing.ts
import { defineRouting } from "next-intl/routing";
import { createNavigation } from "next-intl/navigation";

export const routing = defineRouting({
  locales: [
    "en-NG",      // Nigerian English
    "pcm-NG",     // Nigerian Pidgin (Naija)
    "fr-CM",      // Cameroonian French
    "ha-NG-Arab"  // Hausa Ajami (RTL)
  ] as const,
  defaultLocale: "en-NG",
  localePrefix: "always",
});

export type Locale = (typeof routing.locales)[number];

export const { Link, redirect, usePathname, useRouter } = createNavigation(routing);