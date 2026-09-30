// app/[locale]/courtroom/[deck_id]/page.tsx
import { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { CourtroomClient, SquadMember, Question } from "./CourtroomClient";

export async function generateMetadata({
  params,
}: {
  params: Promise<{ locale: string }>;
}): Promise<Metadata> {
  const { locale } = await params;
  const t = await getTranslations({ locale, namespace: "Courtroom" });

  return { title: `${t("pageTitle")} | CAST` };
}

export default async function CourtroomPage({
  params,
}: {
  params: Promise<{ locale: string; deck_id: string }>;
}) {
  const { deck_id } = await params;

  // 1. MOCK BACKEND DATA
  const squadName = "The Lagos Four";
  
  const mockMembers: SquadMember[] = [
    { id: "u1", name: "Amaka", handle: "@amaka_x", avatar: "https://i.pravatar.cc/150?img=47" },
    { id: "u2", name: "Miriam", handle: "@mimibaby", avatar: "https://i.pravatar.cc/150?img=12" },
    { id: "u3", name: "Emeka", handle: "@emeka_don", avatar: "https://i.pravatar.cc/150?img=11" },
    { id: "u4", name: "Kolade", handle: "@kay_money", avatar: "https://i.pravatar.cc/150?img=14" },
  ];

  const mockQuestions: Question[] = [
    { id: "q1", prompt: "Most likely to ghost their own wedding" },
    { id: "q2", prompt: "Most likely to steal a boyfriend from her bestie" },
    { id: "q3", prompt: "Secretly has a burner account to stalk their ex" },
    { id: "q4", prompt: "Most likely to fake a lifestyle for the gram" },
  ];

  // In production, we generate a unique session_id when they start voting
  const mockSessionId = "session-123"; 

  return (
    <main className="fixed inset-0 bg-[#080808] text-[#F0EDE8] overflow-hidden selection:bg-[#E8390E] selection:text-white">
      <CourtroomClient 
        deckId={deck_id}
        squadName={squadName}
        members={mockMembers} 
        questions={mockQuestions} 
        sessionId={mockSessionId}
      />
    </main>
  );
}