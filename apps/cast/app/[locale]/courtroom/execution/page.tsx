// app/[locale]/courtroom/execution/page.tsx
import { Metadata } from "next";
import { ExecutionClient } from "./ExecutionClient";

export const metadata: Metadata = {
  title: "Verdict Sealed | CAST",
};

export default async function ExecutionPage() {
  // In production, you will fetch the real data here via RPC:
  // const data = await rpc.getVerdictResult({ sessionId: "..." })
  
  // High-fidelity mock data mimicking the Visual Bible
  const mockVerdictData = {
    targetName: "Amaka",
    targetImage: "https://i.pravatar.cc/150?img=47", 
    prompt: "MOST LIKELY TO GHOST THEIR OWN WEDDING",
    intensityScore: 9.6,
    yesPercentage: 70,
    noPercentage: 30,
    totalVotes: 10,
    yesVotes: 7,
    voters: [
      { id: "1", avatar: "https://i.pravatar.cc/150?img=11", hasVoted: true },
      { id: "2", avatar: "https://i.pravatar.cc/150?img=12", hasVoted: true },
      { id: "3", avatar: "https://i.pravatar.cc/150?img=13", hasVoted: true },
      { id: "4", avatar: "https://i.pravatar.cc/150?img=14", hasVoted: true },
      { id: "5", avatar: "https://i.pravatar.cc/150?img=15", hasVoted: false },
      { id: "6", avatar: "https://i.pravatar.cc/150?img=16", hasVoted: false },
      { id: "7", avatar: "https://i.pravatar.cc/150?img=17", hasVoted: true },
    ]
  };

  return (
    <main className="min-h-screen bg-[#080808] text-[#F0EDE8] overflow-hidden selection:bg-[#E8390E] selection:text-white">
      {/* We pass the server-fetched data into our Client orchestrator */}
      <ExecutionClient verdict={mockVerdictData} />
    </main>
  );
}