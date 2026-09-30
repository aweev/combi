// apps/cast/i18n/registry.ts
export type Direction = "ltr" | "rtl";
export type Currency = "NGN" | "CFA" | "GHS";

export interface LocaleMeta {
    tag: string;
    direction: Direction;
    currency: Currency;
    currencySymbol: string;
    nativeName: string;
    isFormal: boolean;
}

export const localeRegistry: Record<string, LocaleMeta> = {
    "en-NG": {
        tag: "en-NG",
        direction: "ltr",
        currency: "NGN",
        currencySymbol: "₦",
        nativeName: "English (Nigeria)",
        isFormal: true,
    },
    "pcm-NG": {
        tag: "pcm-NG",
        direction: "ltr",
        currency: "NGN",
        currencySymbol: "₦",
        nativeName: "Naija",
        isFormal: false,
    },
    "fr-CM": {
        tag: "fr-CM",
        direction: "ltr",
        currency: "CFA",
        currencySymbol: "CFA",
        nativeName: "Français (Cameroun)",
        isFormal: true,
    },
    "ha-NG-Arab": {
        tag: "ha-NG-Arab",
        direction: "rtl",
        currency: "NGN",
        currencySymbol: "₦",
        nativeName: "هَوْسَ",
        isFormal: true,
    },
};

// Helpers
export function getLocaleMeta(lang: string): LocaleMeta {
    return localeRegistry[lang] || localeRegistry["en-NG"];
}

export function formatMoney(amount: number, meta: LocaleMeta): string {
    if (meta.currency === "CFA") {
        // CFA display usually appends the currency name without decimal points
        return `${amount} CFA`;
    }
    return `${meta.currencySymbol}${amount.toLocaleString()}`;
}