// app/[locale]/courtroom/execution/VerdictHero.tsx
"use client";

import { motion, Variants } from "framer-motion";
import Image from "next/image";
import { VerdictData } from "./ExecutionClient"; // We will export this type from the client
import { useTranslations } from "next-intl";

// We explicitly type this as Variants to kill the TypeScript ease array error forever
const heroVariants: Variants = {
  hidden: { opacity: 0, scale: 1.05 },
  show: {
    opacity: 1,
    scale: 1,
    transition: {
      duration: 1.2,
      ease: [0.16, 1, 0.3, 1], // Dramatic curve
    },
  },
};

const textVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1], delay: 0.3 },
  },
};

export function VerdictHero({ verdict }: { verdict: VerdictData }) {
  const t = useTranslations("Execution");
  return (
    <motion.div
      variants={heroVariants}
      className="relative w-full h-[55vh] min-h-112.5 flex flex-col justify-end overflow-hidden"
    >
      {/* 1. The Mugshot (Full Bleed Background) */}
      <div className="absolute inset-0 z-0">
        <Image
          src={verdict.targetImage}
          alt={verdict.targetName}
          fill
          priority
          className="object-cover object-top filter contrast-125 saturate-50"
          sizes="100vw"
        />
      </div>

      {/* 2. The Gradients (Melting the image into Void Black) */}
      {/* Top red cinematic glare */}
      <div className="absolute top-0 left-0 right-0 h-32 bg-linear-to-b from-[#E8390E]/20 to-transparent z-10 mix-blend-overlay" />
      
      {/* Bottom heavy fade into Ground (#080808) */}
      <div className="absolute inset-0 bg-linear-to-t from-[#080808] via-[#080808]/80 to-transparent z-10" />
      <div className="absolute inset-0 bg-linear-to-t from-[#080808] via-transparent to-transparent z-10 h-full" />

      {/* 3. The Overlay Content */}
      <div className="relative z-20 px-6 pb-8 w-full flex flex-col items-center text-center">
        <motion.div variants={textVariants} className="w-full flex justify-between items-end mb-4">
          <div className="text-left">
            <div className="font-mono text-[10px] text-[#E8390E] tracking-[0.15em] uppercase font-bold mb-1">
              {t("targetAcquired")}
            </div>
            <div className="font-serif italic text-[22px] text-[#9A9590]">
              {verdict.targetName}
            </div>
          </div>
          <div className="text-right">
            <div className="font-mono text-[10px] text-[#9A9590] tracking-widest uppercase mb-1">
               {t("finalTally")}
            </div>
            <div className="font-display font-extrabold text-3xl text-[#F0EDE8] leading-none drop-shadow-lg">
              <span className="text-[#E8390E] text-4xl">{verdict.yesVotes}</span>
              <span className="text-text-tertiary">/{verdict.totalVotes}</span>
            </div>
          </div>
        </motion.div>

        <motion.div variants={textVariants} className="w-full h-px bg-white/10 mb-6" />

        <motion.div variants={textVariants}>
          <div className="font-mono text-[11px] text-[#9A9590] tracking-widest uppercase mb-3">
            {t("mostLikelyTo")}
          </div>
          <h2 className="font-display text-[32px] sm:text-[36px] font-extrabold leading-[0.95] tracking-[-0.04em] text-[#F0EDE8] uppercase drop-shadow-2xl">
            {verdict.prompt}
          </h2>
        </motion.div>
      </div>
    </motion.div>
  );
}