// apps/cast/lib/utils/phone.ts
import { Locale } from "@/i18n/routing";

export interface PhoneAutoDetect {
    locale: Locale;
    countryCode: string;
}

/**
 * Parses a phone number prefix to suggest the best starting locale
 * and register currency.
 */
export function detectLocaleFromPhone(phone: string): PhoneAutoDetect {
    const sanitized = phone.trim().replace(/[^0-9+]/g, "");

    if (sanitized.startsWith("+234")) {
        return { locale: "en-NG", countryCode: "NG" }; // Nigeria default
    }
    if (sanitized.startsWith("+237")) {
        return { locale: "fr-CM", countryCode: "CM" }; // Cameroon default
    }
    // Fallback to primary platform market
    return { locale: "en-NG", countryCode: "NG" };
}