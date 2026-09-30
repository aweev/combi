// app/[locale]/(orbit)/decks/page.tsx
import { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { DecksClient, DeckData } from "./DecksClient";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "Decks" }); // Change namespace to Decks

  return {
    title: `${t("pageTitle")} | CAST`, // Actually use 't' here!
  };
}

export default async function DecksPage() {
  // 1. FETCH DYNAMIC DATA FROM BACKEND (Mocked for now)
  // const globalCastCount = await rpc.getGlobalCastCount();
  // const pendingBurnBookCount = await rpc.getPendingBurnBookCount({ userId });
  // const decks = await rpc.getDecks();
  
  const globalCastCount = 2847193; // Will be formatted by the client's locale
  const pendingBurnBookCount = 14;

  const mockDecks: DeckData[] = [
    {
      id: "deck-1",
      title: "The Unilag Burner",
      category: "Chaos",
      playCount: 2341,
      coverImage: "https://images.unsplash.com/photo-1542382257-80dedb725088?q=80&w=500&auto=format&fit=crop", 
      creatorName: "@burner_hq",
      isCreatorVerified: true,
      isTrending: true, // Will span full width
    },
    {
      id: "deck-2",
      title: "Corporate Snakes",
      category: "Sponsored",
      playCount: 1892,
      coverImage: "https://images.unsplash.com/photo-1507679799987-c73779587ccf?q=80&w=500&auto=format&fit=crop",
      sponsorName: "Corporate Wolves",
      isPremium: true, // Will have the padlock
    },
    {
      id: "deck-3",
      title: "Weekend Confessions",
      category: "Sponsored",
      playCount: 3776,
      coverImage: "https://images.unsplash.com/photo-1566737236500-c8ac43014a67?q=80&w=500&auto=format&fit=crop",
      sponsorName: "Jameson",
      isCreatorVerified: true,
      isTrending: true,
    },
    {
      id: "deck-4",
      title: "LASU Undercover",
      category: "Campus",
      playCount: 1105,
      coverImage: "https://images.unsplash.com/photo-1541339907198-e08756dedf3f?q=80&w=500&auto=format&fit=crop",
      creatorName: "@lasu_confessions",
      isCreatorVerified: true,
      isPremium: true,
    },
    {
      id: "deck-5",
      title: "Abuja Streets",
      category: "Creator",
      playCount: 983,
      coverImage: "https://images.unsplash.com/photo-1519671482749-fd09be7ccebf?q=80&w=500&auto=format&fit=crop",
      creatorName: "@abuja_streets",
      isCreatorVerified: true,
    }
  ];

  return (
    <main className="min-h-screen bg-[#080808] text-[#F0EDE8] overflow-hidden selection:bg-[#E8390E] selection:text-white pb-32">
      <DecksClient 
        decks={mockDecks} 
        globalCastCount={globalCastCount}
        pendingBurnBookCount={pendingBurnBookCount}
      />
    </main>
  );
}