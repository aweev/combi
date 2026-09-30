// app/[locale]/courtroom/execution/ExecutionClient.tsx
"use client";

import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslations } from "next-intl";
import { Zap, ShieldAlert } from "lucide-react";

import { VerdictHero } from "./VerdictHero";
import { JuryGrid } from "./JuryGrid";
import { IntensityBar } from "./IntensityBar";
import { SharecardCarousel } from "./SharecardCarousel";
import { ExecutionFooter } from "./ExecutionFooter";

// Bible Motion Specs
const PARANOID_EASE = [0.25, 0.46, 0.45, 0.94];
const CINEMATIC_EASE = [0.76, 0, 0.24, 1];

export type VerdictData = {
  targetName: string;
  targetImage: string;
  prompt: string;
  intensityScore: number;
  yesPercentage: number;
  noPercentage: number;
  totalVotes: number;
  yesVotes: number;
  voters: Array<{ id: string; avatar: string; hasVoted: boolean }>;
};

export function ExecutionClient({ verdict }: { verdict: VerdictData }) {
  const t = useTranslations("Execution");
  const [phase, setPhase] = useState<"SEAL" | "REVEAL">("SEAL");

  useEffect(() => {
    // 1. SUSPENSE PHASE: Trigger haptic pulses
    if (navigator.vibrate) {
      const interval = setInterval(() => navigator.vibrate(20), 1000);
      setTimeout(() => clearInterval(interval), 2800);
    }

    // 2. REVEAL TRANSITION
    const timer = setTimeout(() => {
      setPhase("REVEAL");
      if (navigator.vibrate) navigator.vibrate([100, 50, 200]);
    }, 2800);

    return () => clearTimeout(timer);
  }, []);

  return (
    <div className="relative w-full max-w-md mx-auto h-dvh flex flex-col bg-[#080808] overflow-hidden">
      
      {/* THE CINEMATIC FLASH (Dissolve to White) */}
      <AnimatePresence>
        {phase === "REVEAL" && (
          <motion.div
            initial={{ opacity: 1 }}
            animate={{ opacity: 0 }}
            transition={{ duration: 0.8, ease: "easeOut" }}
            className="fixed inset-0 bg-[#F5F3EE] z-[100] pointer-events-none mix-blend-screen"
          />
        )}
      </AnimatePresence>

      <AnimatePresence mode="wait">
        {/* PHASE 1: THE SEAL (The Holding Cell) */}
        {phase === "SEAL" && (
          <motion.div
            key="seal"
            className="absolute inset-0 flex flex-col items-center justify-center z-50 bg-[#080808]"
            exit={{ opacity: 0, scale: 1.1, filter: "blur(20px)" }}
            transition={{ duration: 0.8, ease: CINEMATIC_EASE }}
          >
            <div className="relative">
              <motion.div
                animate={{ 
                  scale: [1, 1.1, 1],
                  rotate: [0, 5, -5, 0]
                }}
                transition={{ duration: 1.4, repeat: Infinity, ease: PARANOID_EASE }}
                className="w-24 h-24 rounded-full bg-[#E8390E] flex items-center justify-center shadow-[0_0_60px_rgba(232,57,14,0.4)]"
              >
                <Zap className="w-10 h-10 text-white fill-white" />
              </motion.div>
            </div>
            
            <div className="mt-12 text-center">
              <p className="font-mono text-[11px] tracking-[0.3em] text-[#E8390E] uppercase animate-pulse">
                {t("decryptingTruth")}
              </p>
              <p className="font-serif italic text-[16px] text-[#5A5550] mt-4">
                "The Courtroom is weighing the evidence..."
              </p>
            </div>
          </motion.div>
        )}

        {/* PHASE 2: THE REVEAL (Final Ruling) */}
        {phase === "REVEAL" && (
          <motion.div
            key="reveal"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            className="absolute inset-0 flex flex-col w-full overflow-y-auto no-scrollbar"
          >
            <VerdictHero verdict={verdict} />

            <motion.div 
              initial={{ opacity: 0, y: 30 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: 0.4, duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
              className="px-5 pb-40 flex flex-col gap-5 mt-2"
            >
              <JuryGrid voters={verdict.voters} totalVotes={verdict.totalVotes} />
              
              <IntensityBar 
                yesPercentage={verdict.yesPercentage} 
                noPercentage={verdict.noPercentage} 
              />
              
              <SharecardCarousel verdict={verdict} />

              <div className="flex items-center gap-3 p-4 bg-[#111111] border border-white/5 rounded-card">
                <ShieldAlert className="text-[#5A5550]" size={18} />
                <p className="font-mono text-[9px] text-[#5A5550] uppercase tracking-widest leading-relaxed">
                  Identity Protection Active: Individual voter data is permanently encrypted.
                </p>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>

      <AnimatePresence>
        {phase === "REVEAL" && <ExecutionFooter />}
      </AnimatePresence>
    </div>
  );
}

// app/[locale]/courtroom/execution/VerdictHero.tsx
export function VerdictHero({ verdict }: { verdict: VerdictData }) {
  const t = useTranslations("Execution");
  return (
    <div className="relative w-full h-[55vh] flex flex-col justify-end overflow-hidden">
      {/* High-Contrast "Mugshot" Image */}
      <Image
        src={verdict.targetImage}
        alt=""
        fill
        priority
        className="object-cover object-top contrast-[1.4] saturate-[0.2]"
      />

      {/* Surface Gradients (Melt to Void) */}
      <div className="absolute top-0 inset-x-0 h-40 bg-gradient-to-b from-[#E8390E]/15 to-transparent z-10" />
      <div className="absolute inset-0 bg-gradient-to-t from-[#080808] via-[#080808]/80 to-transparent z-10" />

      {/* Content */}
      <div className="relative z-20 px-6 pb-8">
        <div className="flex justify-between items-end mb-6 border-b border-white/10 pb-6">
          <div className="text-left">
            <span className="font-mono text-[10px] text-[#E8390E] tracking-widest uppercase block mb-1">
              Verdict Sealed
            </span>
            <h2 className="font-serif italic text-[28px] text-[#F5F3EE]">
              {verdict.targetName}
            </h2>
          </div>
          <div className="text-right">
             <span className="font-mono text-[10px] text-[#5A5550] uppercase tracking-widest block mb-1">
               Agreement
             </span>
             <div className="font-display font-black text-[38px] text-[#F5F3EE] leading-none">
               {verdict.yesVotes}<span className="text-[#5A5550] text-[24px]">/{verdict.totalVotes}</span>
             </div>
          </div>
        </div>

        <div className="text-center">
          <span className="font-mono text-[11px] text-[#5A5550] uppercase tracking-[0.2em] block mb-3">
            {t("mostLikelyTo")}
          </span>
          <h1 className="font-display text-[36px] font-black leading-[0.9] tracking-[-0.04em] text-[#F5F3EE] uppercase">
            {verdict.prompt}
          </h1>
        </div>
      </div>
    </div>
  );
}

// app/[locale]/courtroom/execution/IntensityBar.tsx
export function IntensityBar({ yesPercentage, noPercentage }: { yesPercentage: number, noPercentage: number }) {
  const t = useTranslations("Execution");

  return (
    <div className="bg-[#111111] border border-white/10 rounded-module p-6 flex flex-col gap-6">
      <div className="flex items-center justify-between font-display font-black text-[32px]">
        <div className="text-[#22C55E]">
          {yesPercentage}<span className="text-[16px]">%</span>
          <span className="block font-mono text-[9px] text-[#5A5550] uppercase tracking-widest mt-1">Ruled Yes</span>
        </div>
        
        <div className="w-10 h-10 rounded-full bg-[#080808] border border-white/5 flex items-center justify-center font-mono italic text-[10px] text-[#5A5550]">
          VS
        </div>

        <div className="text-[#E8390E] text-right">
          {noPercentage}<span className="text-[16px]">%</span>
          <span className="block font-mono text-[9px] text-[#5A5550] uppercase tracking-widest mt-1">Ruled No</span>
        </div>
      </div>

      <div className="w-full h-2 rounded-full flex overflow-hidden bg-[#080808]">
        <motion.div 
          initial={{ width: 0 }}
          animate={{ width: `${yesPercentage}%` }}
          transition={{ duration: 1.5, ease: [0.16, 1, 0.3, 1] }}
          className="bg-[#22C55E]" 
        />
        <motion.div 
          initial={{ width: 0 }}
          animate={{ width: `${noPercentage}%` }}
          transition={{ duration: 1.5, ease: [0.16, 1, 0.3, 1] }}
          className="bg-[#E8390E]" 
        />
      </div>
    </div>
  );
}

// app/[locale]/courtroom/execution/SharecardCarousel.tsx
export function SharecardCarousel({ verdict }: { verdict: VerdictData }) {
  const t = useTranslations("Execution");

  return (
    <div className="w-full">
      <div className="flex justify-between items-center mb-4">
        <h3 className="font-mono text-[11px] text-[#5A5550] uppercase tracking-widest">
          The Viral Receipt
        </h3>
        <span className="font-serif italic text-[13px] text-[#E8390E] animate-pulse">
          Swipe styles →
        </span>
      </div>

      <div className="flex overflow-x-auto snap-x snap-mandatory gap-4 no-scrollbar -mx-5 px-5">
        
        {/* CARD 1: THE MUGSHOT (RED) */}
        <div className="snap-center shrink-0 w-72 aspect-[9/16] bg-[#E8390E] rounded-module p-8 flex flex-col justify-between relative overflow-hidden">
          <Image src={verdict.targetImage} alt="" fill className="object-cover mix-blend-multiply opacity-60 grayscale contrast-[1.5]" />
          <div className="absolute inset-0 bg-gradient-to-b from-black/40 via-transparent to-black/80" />
          
          <div className="relative z-10">
            <div className="font-display font-black text-[48px] text-white leading-[0.85] uppercase tracking-[-0.05em]">
              THE<br/>VERDICT<br/>IS <span className="text-black">CAST.</span>
            </div>
          </div>

          <div className="relative z-10">
            <p className="font-mono text-[10px] text-white/60 uppercase tracking-widest mb-2">Verdict Against {verdict.targetName}</p>
            <p className="font-display font-black text-[22px] text-white uppercase leading-tight bg-black/30 p-4 backdrop-blur-md">
              "{verdict.prompt}"
            </p>
          </div>
        </div>

        {/* CARD 2: THE RECEIPT (BONE WHITE) */}
        <div className="snap-center shrink-0 w-72 aspect-[9/16] bg-[#F5F3EE] rounded-module p-8 flex flex-col items-center text-center">
          <div className="font-display font-black text-[64px] text-[#080808] leading-none mb-2">
            {verdict.yesVotes}/{verdict.totalVotes}
          </div>
          <p className="font-mono text-[11px] text-[#E8390E] font-bold uppercase tracking-[0.2em] border-b border-black/10 w-full pb-4 mb-8">
            Squad Consensus
          </p>

          <div className="w-32 h-32 rounded-full overflow-hidden border-4 border-[#080808] mb-8 grayscale contrast-125">
             <Image src={verdict.targetImage} alt="" width={128} height={128} className="object-cover h-full" />
          </div>

          <p className="font-serif italic text-[28px] text-[#080808] leading-tight">
            "{verdict.prompt}"
          </p>
          
          <div className="mt-auto pt-8 border-t border-black/10 w-full flex justify-between items-center font-display font-black text-[#080808]">
            <span>CAST</span>
            <span className="font-mono text-[10px]">CAST.GG/JOIN</span>
          </div>
        </div>
      </div>
    </div>
  );
}