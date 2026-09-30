// app/[locale]/courtroom/results/[session_id]/ResultsClient.tsx
"use client";

import Image from "next/image";
import { useLocale, useTranslations } from "next-intl";
import { motion, Variants } from "framer-motion";
import { Share, RotateCcw } from "lucide-react";
import { Link } from "@/i18n/routing";

export type TopVotedUser = {
  name: string;
  handle: string;
  avatar: string;
  voteCount: number;
  percentage: number;
};

export type AggregateTally = {
  id: string;
  prompt: string;
  topVoted: TopVotedUser;
};

export type SessionResultData = {
  sessionId: string;
  deckName: string;
  squadName: string;
  totalParticipants: number;
  completedAt: string;
  tallies: AggregateTally[];
};

// ── MOTION VARIANTS ────────────────────────────────────
const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.1, delayChildren: 0.2 },
  },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.6, ease: [0.16, 1, 0.3, 1] }, // Dramatic curve
  },
};

const headerVariants: Variants = {
  hidden: { opacity: 0, scale: 0.95 },
  show: {
    opacity: 1,
    scale: 1,
    transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] },
  },
};

export function ResultsClient({ data }: { data: SessionResultData }) {
  const t = useTranslations("Results");
  const locale = useLocale();
  
  const handleShareDeck = async () => {
    const baseUrl = typeof window !== "undefined" ? window.location.origin : "";
    
    // Inject the locale into the viral link
    const shareUrl = `${baseUrl}/${locale}/vote/${data.sessionId}`;

    if (navigator.share) {
      try {
        await navigator.share({
          title: data.deckName,
          text: t("shareDeckText", { deckName: data.deckName }),
          url: shareUrl, 
        });
      } catch (error) {
        console.log("Error sharing:", error);
      }
    } else {
      navigator.clipboard.writeText(shareUrl);
      alert(t("copiedSuccess"));
    }
  };

  return (
    <div className="w-full max-w-md mx-auto relative min-h-screen">
      <div className="px-5 pt-12 flex flex-col gap-6 pb-32">
        
        {/* HEADER SECTION */}
        <motion.header 
          variants={headerVariants}
          initial="hidden"
          animate="show"
          className="flex flex-col gap-2 mb-4 text-start"
        >
          <div className="font-mono text-[10px] text-[#E8390E] tracking-[0.15em] uppercase font-bold flex items-center gap-2">
            <span className="w-1.5 h-1.5 rounded-full bg-[#E8390E] animate-pulse" />
            {t("sessionComplete")}
          </div>
          
          <h1 className="font-display text-[44px] font-extrabold leading-[0.9] tracking-[-0.03em] uppercase text-[#F0EDE8]">
            {t("theTruth")} <br />
            <span className="text-[#9A9590] italic font-serif tracking-normal lowercase text-[36px]">
              {t("isOut")}
            </span>
          </h1>
          
          <div className="mt-2 flex items-center gap-3 font-mono text-[11px] text-text-tertiary uppercase tracking-widest">
            <span>{data.squadName}</span>
            <span className="w-1 h-1 rounded-full bg-text-tertiary" />
            <span>{t("players", { count: data.totalParticipants })}</span>
          </div>
        </motion.header>

        <hr className="border-t border-white/10" />

        {/* TALLIES LIST */}
        <motion.div 
          variants={containerVariants}
          initial="hidden"
          animate="show"
          className="flex flex-col gap-4"
        >
          {data.tallies.map((tally, index) => (
            <motion.div 
              variants={itemVariants}
              key={tally.id}
              className="bg-[#111111] border border-white/10 rounded-module p-5 flex flex-col gap-4 shadow-lg"
            >
              {/* Prompt */}
              <div className="font-mono text-[10px] text-[#9A9590] tracking-widest uppercase text-start">
                {t("question", { num: (index + 1).toString().padStart(2, "0") })}
              </div>
              <h3 className="font-display font-extrabold text-[20px] leading-[1.1] text-[#F0EDE8] uppercase text-start">
                {tally.prompt}
              </h3>
              
              {/* Winner Plate */}
              <div className="mt-2 bg-[#1A1A1A] border border-white/5 rounded-card p-3 flex items-center gap-4 relative overflow-hidden">
                {/* Subtle background progress bar based on vote percentage */}
                <div 
                  className="absolute inset-y-0 inset-s-0 bg-white/2 z-0"
                  style={{ width: `${tally.topVoted.percentage}%` }}
                />

                <div className="relative z-10 w-12 h-12 rounded-full border border-white/10 overflow-hidden shrink-0">
                  <Image 
                    src={tally.topVoted.avatar} 
                    alt={tally.topVoted.name} 
                    fill 
                    className="object-cover filter contrast-125 saturate-50"
                  />
                </div>
                
                <div className="relative z-10 flex-1 flex flex-col text-start">
                  <span className="font-serif italic text-[20px] text-[#F0EDE8] leading-none">
                    {tally.topVoted.name}
                  </span>
                  <span className="font-mono text-[10px] text-text-tertiary">
                    {tally.topVoted.handle}
                  </span>
                </div>
                
                <div className="relative z-10 flex flex-col items-end text-end">
                  <span className="font-display font-bold text-[24px] text-[#E8390E] leading-none drop-shadow-md">
                    {tally.topVoted.voteCount}
                  </span>
                  <span className="font-mono text-[9px] text-[#9A9590] uppercase tracking-widest">
                    {t("votes")}
                  </span>
                </div>
              </div>
            </motion.div>
          ))}
        </motion.div>
      </div>

      {/* STICKY BOTTOM ACTION BAR */}
      <motion.div 
        className="fixed bottom-0 left-0 right-0 p-5 z-50 max-w-md mx-auto"
        initial={{ opacity: 0, y: 100 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ delay: 0.8, duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
      >
        {/* Dark gradient to mask scroll area cleanly */}
        <div className="absolute inset-0 bg-linear-to-t from-[#080808] via-[#080808]/95 to-transparent -z-10 pointer-events-none h-[150%]" />
        
        <div className="flex gap-3 relative z-10">
          <Link href="/decks" className="flex-1 bg-transparent border border-white/20 text-[#F0EDE8] rounded-[4px] h-14 font-display font-bold text-[13px] tracking-[0.02em] uppercase flex items-center justify-center gap-2 hover:bg-white/5 transition-colors backdrop-blur-md">
            <RotateCcw className="w-4 h-4" /> {t("playAgain")}
          </Link>
          <button onClick={handleShareDeck} className="flex-1 bg-[#E8390E] text-white rounded-[4px] h-14 font-display font-bold text-[13px] tracking-[0.02em] uppercase flex items-center justify-center gap-2 hover:bg-[#C43209] transition-all shadow-[0_0_20px_rgba(232,57,14,0.3)] active:scale-95">
            <Share className="w-4 h-4 fill-white" /> {t("shareDeck")}
          </button>
        </div>
      </motion.div>

    </div>
  );
}