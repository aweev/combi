// app/[locale]/courtroom/execution/IntensityBar.tsx
"use client";

import { motion, Variants } from "framer-motion";
import { useTranslations } from "next-intl";

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 20 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.8, ease: [0.16, 1, 0.3, 1] },
  },
};

export function IntensityBar({ yesPercentage, noPercentage }: { yesPercentage: number, noPercentage: number }) {
  const t = useTranslations("Execution");

  return (
    <motion.div 
      variants={itemVariants} 
      className="bg-[#111111] border border-white/10 rounded-module p-5 flex flex-col gap-5 shadow-lg relative"
    >
      <div className="flex items-center justify-between font-display font-extrabold text-[26px] leading-none">
        
        {/* YES Side */}
        <div className="text-success flex flex-col">
          {yesPercentage}% 
          <span className="text-[9px] font-mono font-medium tracking-widest text-[#9A9590] uppercase mt-1.5">
            {t("agreed")}
          </span>
        </div>
        
        {/* VS Badge */}
        <div className="w-9 h-9 rounded-full bg-[#1A1A1A] border border-white/10 flex items-center justify-center shadow-inner">
          <span className="font-mono text-[10px] italic text-text-tertiary tracking-widest pl-0.5">
            VS
          </span>
        </div>
        
        {/* NO Side */}
        <div className="text-[#E8390E] flex flex-col items-end">
          {noPercentage}% 
          <span className="text-[9px] font-mono font-medium tracking-widest text-[#9A9590] uppercase mt-1.5">
            {t("denied")}
          </span>
        </div>

      </div>

      {/* The Animated Bar */}
      <div className="w-full h-2.5 rounded-full flex overflow-hidden bg-[#1A1A1A] shadow-inner">
        <motion.div 
          className="bg-success h-full"
          initial={{ width: "0%" }}
          animate={{ width: `${yesPercentage}%` }}
          transition={{ duration: 1.2, ease: [0.16, 1, 0.3, 1], delay: 0.3 }}
        />
        <motion.div 
          className="bg-[#E8390E] h-full"
          initial={{ width: "0%" }}
          animate={{ width: `${noPercentage}%` }}
          transition={{ duration: 1.2, ease: [0.16, 1, 0.3, 1], delay: 0.3 }}
        />
      </div>
    </motion.div>
  );
}