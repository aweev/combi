// app/[locale]/(orbit)/home/HomeClient.tsx
"use client";

import { useTranslations } from "next-intl";
import { motion, Variants } from "framer-motion";
import { Link } from "@/i18n/routing";
import { Flame, Lock, ChevronRight, Activity } from "lucide-react";

export type DailyPrompt = {
  id: string;
  dayNumber: number;
  prompt: string;
};

export type PendingVerdict = {
  id: string;
  icon: string;
  blurredText: string;
};

interface HomeClientProps {
  dailyPrompt: DailyPrompt;
  pendingVerdicts: PendingVerdict[];
}

// ── MOTION VARIANTS ──
const containerVariants: Variants = {
  hidden: { opacity: 0 },
  show: {
    opacity: 1,
    transition: { staggerChildren: 0.1 },
  },
};

const itemVariants: Variants = {
  hidden: { opacity: 0, y: 15 },
  show: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.5, ease: [0.16, 1, 0.3, 1] },
  },
};

export function HomeClient({ dailyPrompt, pendingVerdicts }: HomeClientProps) {
  const t = useTranslations("Home");

  return (
    <motion.div 
      variants={containerVariants}
      initial="hidden"
      animate="show"
      className="w-full max-w-md mx-auto px-5 pt-6 pb-32 flex flex-col gap-8"
    >
      
      {/* 1. THE DAILY CHAOS PROMPT (The Oracle) */}
      <motion.div variants={itemVariants}>
        <Link 
          href={`/courtroom/${dailyPrompt.id}`}
          className="block w-full bg-[#E8390E]/10 border border-[#E8390E]/30 rounded-[12px] p-4 relative overflow-hidden group active:scale-[0.98] transition-transform"
        >
          {/* Subtle animated gradient background */}
          <div className="absolute inset-0 bg-gradient-to-br from-[#E8390E]/5 to-transparent pointer-events-none" />
          
          <div className="relative z-10 flex items-center justify-between mb-2">
            <div className="font-mono text-[10px] text-[#E8390E] font-bold tracking-[0.1em] uppercase flex items-center gap-1.5">
              <Flame className="w-3.5 h-3.5" />
              {t("todaysChaos")} · {t("day", { num: dailyPrompt.dayNumber })}
            </div>
          </div>
          
          <h2 className="relative z-10 font-display text-[22px] font-extrabold text-[#F0EDE8] leading-[1.1] tracking-[-0.02em] group-hover:text-white transition-colors">
            "{dailyPrompt.prompt}"
          </h2>
          
          <div className="relative z-10 mt-4 flex items-center justify-between">
            <span className="font-mono text-[10px] text-[#E8390E] tracking-widest uppercase">
              Play Now
            </span>
            <ChevronRight className="w-4 h-4 text-[#E8390E] group-hover:translate-x-1 rtl:group-hover:-translate-x-1 transition-transform" />
          </div>
        </Link>
      </motion.div>

      {/* 2. PENDING VERDICTS (The Burn Book Preview) */}
      <motion.div variants={itemVariants} className="flex flex-col gap-3">
        <div className="flex items-center justify-between">
          <h3 className="font-mono text-[10px] text-[#9A9590] tracking-[0.09em] uppercase">
            {t("pendingVerdicts")}
          </h3>
          <span className="font-mono text-[9px] bg-[#E8390E]/10 text-[#E8390E] px-2 py-0.5 rounded-[4px] uppercase tracking-widest font-bold">
            {pendingVerdicts.length} {t("new")}
          </span>
        </div>

        <div className="flex flex-col gap-2">
          {pendingVerdicts.map((verdict) => (
            <Link
              key={verdict.id}
              href="/profile/burn-book"
              className="bg-[#111111] border border-white/5 hover:border-white/10 rounded-[12px] p-3 flex items-center gap-3 group transition-all active:scale-[0.98]"
            >
              {/* Icon / Lock Box */}
              <div className="w-10 h-10 rounded-[8px] bg-[#E8390E]/10 border border-[#E8390E]/20 flex items-center justify-center text-[16px] shrink-0 relative overflow-hidden">
                <span className="opacity-40">{verdict.icon}</span>
                <div className="absolute inset-0 bg-black/40 backdrop-blur-[2px] flex items-center justify-center">
                  <Lock className="w-4 h-4 text-[#E8390E]" />
                </div>
              </div>

              {/* Blurred Text Area */}
              <div className="flex-1 overflow-hidden text-start">
                <div className="font-mono text-[9px] font-bold text-[#F0EDE8]/80 uppercase tracking-widest mb-1">
                  {t("yourSquadVoted")}
                </div>
                <div className="text-[12px] text-[#9A9590] truncate filter blur-[3.5px] select-none group-hover:blur-[2.5px] transition-all duration-300">
                  {verdict.blurredText}
                </div>
              </div>
            </Link>
          ))}
        </div>
      </motion.div>

      {/* 3. LIVE ACTIVITY FEED */}
      <motion.div variants={itemVariants} className="flex flex-col gap-3 mt-4">
        <h3 className="font-mono text-[10px] text-[#9A9590] tracking-[0.09em] uppercase flex items-center gap-1.5">
          <Activity className="w-3 h-3" /> {t("recentActivity")}
        </h3>
        
        <div className="bg-[#111111] border border-white/5 rounded-[12px] p-4 flex flex-col gap-4">
           {/* Activity Item 1 */}
           <div className="flex gap-3 items-start">
             <div className="w-1.5 h-1.5 rounded-full bg-[#22C55E] mt-1.5 shrink-0" />
             <p className="font-mono text-[11px] text-[#9A9590] leading-relaxed text-start">
               <span className="text-[#F0EDE8] font-bold">@amaka_x</span> just unlocked a verdict in <span className="text-[#F0EDE8]">Love & Lies</span>.
             </p>
           </div>
           
           <hr className="border-t border-white/5" />
           
           {/* Activity Item 2 */}
           <div className="flex gap-3 items-start">
             <div className="w-1.5 h-1.5 rounded-full bg-[#C9A84C] mt-1.5 shrink-0 shadow-[0_0_8px_rgba(201,168,76,0.6)]" />
             <p className="font-mono text-[11px] text-[#9A9590] leading-relaxed text-start">
               <span className="text-[#F0EDE8] font-bold">The Lagos Four</span> just entered the Inner Court.
             </p>
           </div>
        </div>
      </motion.div>
    </motion.div>
    
  );
}