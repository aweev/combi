// app/[locale]/courtroom/execution/JuryGrid.tsx
"use client";

import { motion, Variants } from "framer-motion";
import Image from "next/image";
import { ChevronRight } from "lucide-react";
import { useTranslations } from "next-intl";
import { VerdictData } from "./ExecutionClient";

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] },
  },
};

export function JuryGrid({ voters, totalVotes }: { voters: VerdictData['voters'], totalVotes: number }) {
  const t = useTranslations("Execution");
  const tCommon = useTranslations("Common");
  const displayLimit = 6;
  const visibleVoters = voters.slice(0, displayLimit);
  const remaining = totalVotes - displayLimit;

  return (
    <motion.div 
      variants={itemVariants} 
      className="bg-[#111111] border border-white/10 rounded-module p-5 flex flex-col gap-4 shadow-lg relative overflow-hidden"
    >
      {/* Subtle top glare */}
      <div className="absolute top-0 left-0 right-0 h-px bg-linear-to-r from-transparent via-white/10 to-transparent" />

      <div className="font-mono text-[10px] text-[#9A9590] tracking-widest uppercase flex items-center justify-between">
        <span>{t("theJury")}</span>
        <span className="text-text-tertiary">{t("identityProtected")}</span>
      </div>

      <div className="flex items-center justify-between">
        <div className="flex -space-x-3">
          {visibleVoters.map((voter, i) => (
            <div 
              key={i} 
              className={`relative w-10 h-10 rounded-full border-[2.5px] border-[#111111] overflow-hidden shadow-md transition-all duration-500
                ${voter.hasVoted 
                  ? 'ring-1 ring-[#E8390E]/50 opacity-100 drop-shadow-[0_0_8px_rgba(232,57,14,0.3)]' 
                  : 'opacity-30 grayscale'
                }`}
            >
              <Image 
                src={voter.avatar} 
                alt="Jury Member" 
                width={40} 
                height={40} 
                className="w-full h-full object-cover" 
              />
            </div>
          ))}
          
          {remaining > 0 && (
            <div className="w-10 h-10 rounded-full border-[2.5px] border-[#111111] bg-[#1A1A1A] flex items-center justify-center font-mono text-[10px] text-[#9A9590] z-10 shadow-md">
              +{remaining}
            </div>
          )}
        </div>
        
        <button className="font-mono text-[10px] tracking-widest text-[#F0EDE8] hover:text-[#E8390E] transition-colors flex items-center group py-2">
          {tCommon("viewAll")}
          <ChevronRight className="w-3 h-3 ml-0.5 group-hover:translate-x-0.5 transition-transform" />
        </button>
      </div>
    </motion.div>
  );
}