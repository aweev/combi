// app/[locale]/courtroom/results/[session_id]/page.tsx
import { Metadata } from "next";
import { notFound } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { ResultsClient, SessionResultData } from "./ResultsClient";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string; session_id: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "Results" });

  return {
    title: `${t("pageTitle")} | CAST`,
  };
}

export default async function SessionResultsPage({
  params,
}: {
  params: Promise<{ locale: string; session_id: string }>;
}) {
  const { session_id } = await params;

  if (!session_id) {
    notFound();
  }

  // High-fidelity mock data (to be replaced with your RPC call)
  const mockResults: SessionResultData = {
    sessionId: session_id,
    deckName: "Love & Lies (Unilag Edition)",
    squadName: "The Lagos Four",
    totalParticipants: 4,
    completedAt: new Date().toISOString(),
    tallies: [
      {
        id: "q1",
        prompt: "Most likely to ghost their own wedding",
        topVoted: {
          name: "Amaka",
          handle: "@amaka_x",
          avatar: "https://i.pravatar.cc/150?img=47",
          voteCount: 3,
          percentage: 75,
        },
      },
      {
        id: "q2",
        prompt: "Most likely to steal a boyfriend from her bestie",
        topVoted: {
          name: "Miriam",
          handle: "@mimibaby",
          avatar: "https://i.pravatar.cc/150?img=12",
          voteCount: 2,
          percentage: 50,
        },
      },
      {
        id: "q3",
        prompt: "Secretly has a burner account to stalk their ex",
        topVoted: {
          name: "Emeka",
          handle: "@emeka_don",
          avatar: "https://i.pravatar.cc/150?img=11",
          voteCount: 4,
          percentage: 100,
        },
      },
      {
        id: "q4",
        prompt: "Most likely to fake a lifestyle for the gram",
        topVoted: {
          name: "Kolade",
          handle: "@kay_money",
          avatar: "https://i.pravatar.cc/150?img=14",
          voteCount: 2,
          percentage: 50,
        },
      }
    ]
  };

  return (
    <main className="min-h-screen bg-[#080808] text-[#F0EDE8] overflow-hidden selection:bg-[#E8390E] selection:text-white pb-32">
      <ResultsClient data={mockResults} />
    </main>
  );
}