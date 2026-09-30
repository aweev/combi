// app/[locale]/(orbit)/home/page.tsx
import { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { HomeClient, PendingVerdict, DailyPrompt } from "./HomeClient";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "Home" });

  return { title: `${t("pageTitle")} | CAST` };
}

export default async function HomePage() {
  // 1. MOCK DATA: Fetched from Hermes/Backend
  const mockDailyPrompt: DailyPrompt = {
    id: "daily-12",
    dayNumber: 12,
    prompt: "If your squad had to disappear one member for 30 days, who is it?",
  };

  const mockPendingVerdicts: PendingVerdict[] = [
    {
      id: "v1",
      icon: "⚖️",
      blurredText: "Most likely to steal a boyfriend from her own bestie",
    },
    {
      id: "v2",
      icon: "🔥",
      blurredText: "Most likely to go abroad and forget everyone",
    },
    {
      id: "v3",
      icon: "👁️",
      blurredText: "Secretly has a burner account to stalk their ex",
    }
  ];

  return (
    // Note: We don't need min-h-screen or bg-[#080808] here because 
    // the (orbit)/layout.tsx already handles the app shell!
    <div className="w-full">
      <HomeClient 
        dailyPrompt={mockDailyPrompt} 
        pendingVerdicts={mockPendingVerdicts} 
      />
    </div>
  );
}