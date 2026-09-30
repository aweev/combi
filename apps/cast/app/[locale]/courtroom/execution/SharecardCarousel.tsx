// app/[locale]/courtroom/execution/SharecardCarousel.tsx
"use client";

import { motion, Variants } from "framer-motion";
import Image from "next/image";
import { useTranslations } from "next-intl";
import { VerdictData } from "./ExecutionClient";

const vanityUrl = process.env.NEXT_PUBLIC_VANITY_URL || "cast.gg";

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] },
  },
};

export function SharecardCarousel({ verdict }: { verdict: VerdictData }) {
  const t = useTranslations("Execution");

  return (
    <motion.div variants={itemVariants} className="w-full mt-2">
      <div className="flex justify-between items-end px-1 mb-4">
        <h3 className="font-mono text-[11px] tracking-widest text-[#9A9590] uppercase font-medium">
          {t("shareYourVerdict")}
        </h3>
        <span className="font-serif italic text-[14px] text-text-tertiary animate-pulse">
          {t("swipeStyles")} &rarr;
        </span>
      </div>

      {/* Scroll container: Native touch snapping, zero scrollbars */}
      <div className="flex overflow-x-auto snap-x snap-mandatory gap-4 pb-2 -mx-5 px-5 [&::-webkit-scrollbar]:hidden [-ms-overflow-style:none] scrollbar-none">
        
        {/* SHARECARD 1: The Mugshot (Heavy Red Overlay) */}
        <div className="snap-center shrink-0 w-65 aspect-9/16 bg-[#E8390E] border border-[#B82C08] rounded-module flex flex-col justify-between items-center p-6 relative overflow-hidden shadow-2xl">
          <Image 
            src={verdict.targetImage} 
            alt="Mugshot background" 
            fill 
            className="object-cover opacity-50 mix-blend-multiply contrast-125 saturate-0" 
          />
          <div className="absolute inset-0 bg-linear-to-b from-black/40 via-transparent to-black/90" />
          
          <h4 className="font-display font-extrabold text-[42px] text-white leading-[0.9] text-center tracking-[-0.04em] z-10 mt-4 drop-shadow-xl">
            {t("iGotCasted").split(" ").slice(0, 2).join(" ")} <br/>{t("iGotCasted").split(" ").slice(2).join(" ")}
          </h4>
          
          <div className="z-10 w-full text-center">
            <p className="font-mono text-[9px] text-white/70 uppercase tracking-widest mb-2">
              {t("asThePerson")}
            </p>
            <p className="font-display font-bold text-[20px] text-white uppercase leading-[1.1] bg-black/40 p-4 rounded-xl backdrop-blur-md border border-white/10 shadow-xl">
              {verdict.prompt}
            </p>
            <div className="mt-6 flex justify-between items-center px-2">
               <span className="font-display font-extrabold text-[16px] text-white tracking-widest opacity-80">CAST</span>
               <span className="font-mono text-[9px] text-white/60">{vanityUrl}/join</span>
            </div>
          </div>
        </div>

        {/* SHARECARD 2: The Exposé (Dark & Gritty) */}
        <div className="snap-center shrink-0 w-65 aspect-9/16 bg-[#080808] border border-white/10 rounded-module flex flex-col justify-center items-center p-6 relative overflow-hidden shadow-xl">
          <Image 
            src={verdict.targetImage} 
            alt="Dark background" 
            fill 
            className="object-cover opacity-20 filter grayscale" 
          />
          <div className="absolute inset-0 bg-[radial-gradient(ellipse_70%_50%_at_50%_70%,rgba(232,57,14,0.25)_0%,transparent_70%)]" />
          
          <div className="absolute top-6 w-full text-center font-mono text-[9px] tracking-[0.15em] text-[#E8390E] uppercase drop-shadow-md">
            {t("burnBook")} · {t("entry")} 0{verdict.totalVotes}
          </div>
          
          <div className="z-10 w-full relative">
            <p className="font-mono text-[9px] text-[#9A9590] uppercase tracking-widest mb-3">{t("mySquadVotedMe")}</p>
            <h4 className="font-display font-extrabold text-[32px] text-[#F0EDE8] leading-[0.95] tracking-[-0.03em] uppercase mb-5">
              {t("mostLikelyTo")} <br/><span className="text-[#E8390E]">{verdict.prompt.replace("MOST LIKELY TO ", "")}</span>
            </h4>
            <div className="w-12 h-0.5 bg-[#E8390E] mb-4" />
            <p className="font-serif italic text-[22px] text-[#9A9590] leading-tight">
              {t("theyAreNotWrong")}
            </p>
          </div>

          <div className="absolute bottom-6 left-6 right-6 flex justify-between items-center border-t border-white/10 pt-4">
            <span className="font-display font-extrabold text-[14px] text-[#F0EDE8] tracking-[-0.04em]">CAST</span>
            <span className="font-mono text-[9px] text-text-tertiary">{vanityUrl}/join</span>
          </div>
        </div>

        {/* SHARECARD 3: The Receipt (Clinical & Aggressive) */}
        <div className="snap-center shrink-0 w-65 aspect-9/16 bg-[#F0EDE8] border border-[#080808] rounded-module flex flex-col justify-between items-center p-6 relative overflow-hidden shadow-xl">
          {/* Noise texture for receipt feel */}
          <div className="absolute inset-0 opacity-[0.03] bg-[url('https://grainy-gradients.vercel.app/noise.svg')]" />
          
          <div className="z-10 text-center w-full mt-4">
            <div className="font-display font-extrabold text-4xl text-[#080808] mb-2 tracking-tighter">
              {verdict.yesVotes}/{verdict.totalVotes}
            </div>
            <p className="font-mono text-[10px] text-[#E8390E] font-bold uppercase tracking-widest border-b border-[#080808]/10 pb-4 mb-6">
              {t("squadConsensus")}
            </p>
            
            <div className="w-24 h-24 mx-auto rounded-full overflow-hidden border-[3px] border-[#080808] mb-6 shadow-md filter grayscale contrast-125">
              <Image src={verdict.targetImage} alt="Receipt avatar" width={96} height={96} className="object-cover w-full h-full" />
            </div>

            <p className="font-serif italic text-[26px] text-[#080808] leading-[1.05] mb-2">
              &quot;{verdict.prompt}&quot;
            </p>
          </div>
          
          <div className="w-full flex justify-between items-center border-t border-[#080808]/20 pt-4 z-10">
             <span className="font-display font-extrabold text-[14px] text-[#080808] tracking-[-0.04em]">CAST</span>
             <span className="font-mono text-[9px] text-[#080808]/60 font-bold">{vanityUrl}/join</span>
          </div>
        </div>

      </div>
    </motion.div>
  );
}