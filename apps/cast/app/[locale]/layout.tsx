// app/[locale]/layout.tsx
import type { Metadata, Viewport } from "next";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, getTranslations } from "next-intl/server";
import { notFound } from "next/navigation";
import { Syne, DM_Mono, Instrument_Serif } from "next/font/google";
import { routing, Locale } from "@/i18n/routing";
import { getLocaleMeta } from "@/i18n/registry";
import { Providers } from "./providers";
import "./globals.css";

// ── FONTS ────────────────────────────────────────────
const syne = Syne({
  subsets: ["latin"],
  weight: ["700", "800"],
  variable: "--font-syne",
  display: "swap",
});

const dmMono = DM_Mono({
  subsets: ["latin"],
  weight: ["300", "400", "500"],
  variable: "--font-mono",
  display: "swap",
});

const instrumentSerif = Instrument_Serif({
  subsets: ["latin"],
  weight: "400",
  style: "italic",
  variable: "--font-serif",
  display: "swap",
});

// ── VIEWPORT (Crucial for PWA) ────────────────────────
export async function generateViewport(): Promise<Viewport> {
  return {
    themeColor: "#080808", // Void Black
    width: "device-width",
    initialScale: 1,
    maximumScale: 1,
    userScalable: false, // Prevents zooming on double tap
    interactiveWidget: "resizes-visual", // Prevents keyboard from squishing the UI
  };
}

// ── DYNAMIC METADATA ──────────────────────────────────
export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;

  // Fetch translations specifically for the Metadata namespace
  const t = await getTranslations({ locale, namespace: "Metadata" });

  return {
    title: t("title"),
    description: t("description"),
    manifest: "/manifest.json",
    appleWebApp: {
      capable: true,
      statusBarStyle: "black-translucent",
      title: "CAST", // Keep the OS app name short and static
    },
    // You can also add OpenGraph data here for rich WhatsApp/Twitter sharing
    openGraph: {
      title: t("title"),
      description: t("description"),
      siteName: "CAST",
    }
  };
}

export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

// ── LAYOUT ───────────────────────────────────────────
export default async function LocaleLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}) {
  const { locale } = await params;
  const meta = getLocaleMeta(locale);

  if (!routing.locales.includes(locale as Locale)) {
    notFound();
  }

  const messages = await getMessages();

  return (
    <html
      lang={locale}
      dir={meta.direction} // This single attribute tells the browser to flip the page
      className={`${syne.variable} ${dmMono.variable} ${instrumentSerif.variable} dark`}
      style={{ overscrollBehavior: "none", backgroundColor: "#080808" }}
    >
      <body className="font-mono text-[#9A9590] antialiased bg-[#080808] selection:bg-[#E8390E] selection:text-white">
        <NextIntlClientProvider messages={messages} locale={locale}>
          <Providers>
            {/* 
              By setting dir="rtl" on the HTML tag, Tailwind v4 will 
              automatically flip properties if you use logical utilities.
            */}
            {children}
          </Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}