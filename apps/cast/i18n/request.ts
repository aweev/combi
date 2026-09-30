// apps/cast/i18n/request.ts
import { getRequestConfig } from "next-intl/server";
import { routing, Locale } from "./routing";

export default getRequestConfig(async ({ requestLocale }) => {
  let locale = await requestLocale;

  if (!locale || !routing.locales.includes(locale as Locale)) {
    locale = routing.defaultLocale;
  }

  return {
    locale,
    // We will build these message files in Step 1 based on the Microcopy Bible
    messages: (await import(`../messages/${locale}.json`)).default,
  };
});