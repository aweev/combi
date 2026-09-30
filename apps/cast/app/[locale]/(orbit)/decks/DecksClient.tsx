// app/[locale]/(orbit)/decks/DecksClient.tsx
"use client";

import { useState, useEffect } from "react";
import Image from "next/image";
import { useTranslations, useLocale } from "next-intl";
import { motion, AnimatePresence } from "framer-motion";
import { Link } from "@/i18n/routing";
import { 
  Flame, HeartCrack, Landmark, GraduationCap, 
  Users, Megaphone, HelpCircle, Lock, Play, 
  ChevronRight, Activity, Book, Scale, AlertCircle, RefreshCcw
} from "lucide-react";

// Visual Bible Motion Specs
const SNAPPY_EASE = [0.2, 0, 0, 1];
const DRAMATIC_EASE = [0.16, 1, 0.3, 1];

export type DeckCategory = "All" | "Chaos" | "Love & Lies" | "Campus" | "Money" | "Creator" | "Sponsored";

export type DeckData = {
  id: string;
  title: string;
  category: DeckCategory;
  playCount: number;
  coverImage: string;
  isPremium?: boolean;
  sponsorName?: string;
  creatorName?: string;
  isCreatorVerified?: boolean;
  isTrending?: boolean;
};

const catConfig: Record<DeckCategory, { i18n: string; icon: any }> = {
  "All": { i18n: "catAll", icon: null },
  "Chaos": { i18n: "catChaos", icon: Flame },
  "Love & Lies": { i18n: "catLoveLies", icon: HeartCrack },
  "Money": { i18n: "catMoney", icon: Landmark },
  "Campus": { i18n: "catCampus", icon: GraduationCap },
  "Creator": { i18n: "catCreator", icon: Users },
  "Sponsored": { i18n: "catSponsored", icon: Megaphone },
};

interface DecksClientProps {
  decks: DeckData[];
  globalCastCount: number;
  pendingBurnBookCount: number;
  isInnerCourtMember?: boolean; // New logic from Product Bible
}

export function DecksClient({ 
  decks, 
  globalCastCount, 
  pendingBurnBookCount, 
  isInnerCourtMember = false 
}: DecksClientProps) {
  const t = useTranslations("Decks");
  const locale = useLocale();
  const [activeCategory, setActiveCategory] = useState<DeckCategory>("All");
  const [showTooltip, setShowTooltip] = useState(false);
  const [isLoading, setIsLoading] = useState(true);

  // 1. STATE: Simulation of initialization (The "Paranoid" Wait)
  useEffect(() => {
    const timer = setTimeout(() => setIsLoading(false), 800);
    return () => clearTimeout(timer);
  }, []);

  const formattedCastCount = new Intl.NumberFormat(locale).format(globalCastCount);
  const categories = Object.keys(catConfig) as DeckCategory[];
  
  // 2. EDGE CASE: Filtering logic
  const filteredDecks = decks.filter(d => activeCategory === "All" ? true : d.category === activeCategory);

  return (
    <div className="w-full max-w-md mx-auto relative min-h-screen pt-8 flex flex-col gap-8 pb-32">
      
      {/* HEADER SECTION */}
      <header className="px-5 text-start relative">
        <motion.div 
          initial={{ opacity: 0, x: -20 }} 
          animate={{ opacity: 1, x: 0 }}
          className="flex justify-between items-start"
        >
          <h1 className="font-display text-[52px] font-extrabold leading-[0.85] tracking-[-0.04em] uppercase text-[#F0EDE8]">
            {t("chooseYour")} <br />
            <span className="text-[#E8390E]">{t("weapon")}</span>
          </h1>
          
          <button 
            onClick={() => setShowTooltip(true)}
            className="mt-2 w-10 h-10 rounded-full border border-white/10 bg-[#111111] flex items-center justify-center text-[#5A5550]"
          >
            <HelpCircle className="w-5 h-5" />
          </button>
        </motion.div>

        <div className="mt-6 flex flex-col gap-1">
          <div className="flex items-center gap-2 font-mono text-[11px] text-[#E8390E] tracking-[0.14em] uppercase">
            <span className="w-2 h-2 rounded-full bg-[#E8390E] animate-pulse shadow-[0_0_8px_#E8390E]" />
            {formattedCastCount} {t("truthsCastToday")}
          </div>
          <p className="font-serif italic text-[#9A9590] text-[15px]">"The truth is blurred. Unlock it."</p>
        </div>
      </header>

      {/* CATEGORY SCROLLER */}
      <div className="px-5 flex overflow-x-auto no-scrollbar gap-3">
        {categories.map((cat) => (
          <button
            key={cat}
            onClick={() => setActiveCategory(cat)}
            className={`flex-shrink-0 flex items-center gap-2 px-4 py-2.5 rounded-[4px] font-mono text-[10px] font-bold uppercase tracking-widest border transition-all duration-150 ease-snappy ${
              activeCategory === cat
                ? "bg-[#E8390E] border-[#E8390E] text-white"
                : "bg-[#111111] border-white/5 text-[#5A5550]"
            }`}
          >
            {catConfig[cat].icon && <span className="w-3.5 h-3.5"><catConfig[cat].icon size={14} /></span>}
            {t(catConfig[cat].i18n)}
          </button>
        ))}
      </div>

      {/* DECK LISTING AREA */}
      <div className="px-5 flex flex-col gap-6">
        <AnimatePresence mode="wait">
          {isLoading ? (
            // 3. STATE: Skeleton Loading (Visual Bible "Surface 1")
            <motion.div key="loading" exit={{ opacity: 0 }} className="flex flex-col gap-6">
              {[1, 2, 3].map(i => (
                <div key={i} className="w-full aspect-[16/9] rounded-module bg-[#111111] animate-pulse border border-white/5" />
              ))}
            </motion.div>
          ) : filteredDecks.length > 0 ? (
            // 4. STATE: Main Feed
            <motion.div 
              key="list" 
              initial={{ opacity: 0 }} 
              animate={{ opacity: 1 }} 
              className="flex flex-col gap-6"
            >
              {filteredDecks.map((deck) => {
                const isLocked = deck.isPremium && !isInnerCourtMember;
                return (
                  <Link 
                    key={deck.id}
                    href={isLocked ? "/inner-court" : `/courtroom/${deck.id}`}
                    className="group relative w-full rounded-module border border-white/10 bg-[#111111] overflow-hidden flex flex-col aspect-[16/9] transition-all duration-150 active:scale-[0.98]"
                  >
                    <Image 
                      src={deck.coverImage} alt="" fill 
                      className={`object-cover opacity-20 grayscale transition-all duration-500 group-hover:scale-105 group-hover:grayscale-0 ${isLocked ? 'blur-xl' : ''}`} 
                    />
                    <div className="absolute inset-0 bg-gradient-to-t from-[#080808] via-transparent to-transparent z-10" />

                    <div className="relative z-20 p-6 h-full flex flex-col">
                      <div className="flex justify-between items-start">
                        <span className="font-mono text-[10px] text-[#5A5550] uppercase tracking-widest">
                          {deck.sponsorName ? `Presented by ${deck.sponsorName}` : deck.creatorName}
                        </span>
                        {deck.isPremium && (
                          <div className={`px-2 py-1 rounded-[2px] border font-mono text-[9px] font-bold uppercase flex items-center gap-1.5 ${
                            isInnerCourtMember ? 'border-green-500/30 text-green-500' : 'border-[#C9A84C]/30 text-[#C9A84C]'
                          }`}>
                            {isInnerCourtMember ? '✓ Access Granted' : '★ Inner Court'}
                          </div>
                        )}
                      </div>

                      <div className="mt-auto">
                        <h3 className={`font-display font-extrabold text-[32px] leading-[0.9] tracking-[-0.04em] uppercase transition-all duration-700 ${
                          isLocked ? "blur-md select-none opacity-40" : "text-[#F0EDE8]"
                        }`}>
                          {deck.title}
                        </h3>
                        <div className="mt-4 flex justify-between items-center font-mono text-[10px] text-[#5A5550]">
                          <span>Cast {new Intl.NumberFormat(locale).format(deck.playCount)} times</span>
                          {!isLocked && <Play size={14} className="text-[#E8390E] fill-[#E8390E]" />}
                        </div>
                      </div>
                    </div>
                  </Link>
                );
              })}
            </motion.div>
          ) : (
            // 5. STATE: Empty State (Provocateur Tone)
            <motion.div 
              key="empty" 
              initial={{ opacity: 0, y: 10 }} 
              animate={{ opacity: 1, y: 0 }}
              className="py-20 text-center flex flex-col items-center gap-4 px-10"
            >
              <AlertCircle className="text-[#5A5550] w-10 h-10" />
              <h4 className="font-display text-[20px] font-bold text-[#F0EDE8] uppercase">{t("catEmptyTitle", { defaultValue: "Your campus is quiet" })}</h4>
              <p className="font-serif italic text-[#9A9590] text-[16px]">{t("catEmptyDesc", { defaultValue: "Too quiet. Build a deck and break the silence." })}</p>
              <Link href="/creator/builder" className="mt-4 px-6 py-3 bg-[#E8390E] text-white font-display font-bold uppercase text-[12px] rounded-[4px]">
                Create a Deck
              </Link>
            </motion.div>
          )}
        </AnimatePresence>
      </div>

      {/* 6. EDGE CASE: Network / Lag UI */}
      <div className="fixed top-20 left-1/2 -translate-x-1/2 z-[60] pointer-events-none">
          {/* Subtle indicator for Campus NAT lag */}
          <div className="flex items-center gap-2 px-3 py-1 bg-black/50 backdrop-blur-md rounded-full border border-white/5 opacity-0 transition-opacity animate-pulse">
            <RefreshCcw size={10} className="text-[#E8390E] animate-spin" />
            <span className="font-mono text-[8px] text-[#9A9590] uppercase tracking-tighter">Syncing Truths...</span>
          </div>
      </div>

      {/* STICKY BURN BOOK TRAP */}
      <AnimatePresence>
        {pendingBurnBookCount > 0 && (
          <motion.div 
            initial={{ y: 100 }} animate={{ y: 0 }}
            className="fixed bottom-24 inset-x-5 z-40 max-w-md mx-auto"
          >
            <Link 
              href="/profile/burn-book"
              className="bg-[#111111] border border-[#E8390E]/30 rounded-[16px] p-5 flex items-center gap-5 shadow-2xl group"
            >
              <div className="relative">
                <div className="w-12 h-12 bg-[#1A1A1A] rounded-[8px] flex items-center justify-center border border-white/10 group-hover:border-[#E8390E]/50 transition-colors">
                  <Book className="w-6 h-6 text-[#5A5550] group-hover:text-[#E8390E]" />
                </div>
                <div className="absolute -top-2 -right-2 w-6 h-6 bg-[#E8390E] rounded-full flex items-center justify-center text-white font-display font-black text-[11px] border-[3px] border-[#111111] animate-pulse">
                  {pendingBurnBookCount}
                </div>
              </div>
              <div className="flex-1">
                <h4 className="font-display font-black text-[18px] text-[#F0EDE8] uppercase italic">{t("yourBurnBook")}</h4>
                <p className="font-mono text-[10px] text-[#E8390E] uppercase tracking-widest">{t("verdictsPending", { count: pendingBurnBookCount })}</p>
              </div>
              <ChevronRight className="text-[#E8390E]" />
            </Link>
          </motion.div>
        )}
      </AnimatePresence>

      {/* TOOLTIP MODAL */}
      <AnimatePresence>
        {showTooltip && (
          <motion.div 
            initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }}
            className="fixed inset-0 z-[100] bg-[#080808]/90 backdrop-blur-md flex items-center justify-center px-6"
            onClick={() => setShowTooltip(false)}
          >
            <motion.div 
              initial={{ scale: 0.9, y: 20 }} animate={{ scale: 1, y: 0 }}
              className="bg-[#111111] border border-white/10 p-8 rounded-module max-w-sm"
              onClick={e => e.stopPropagation()}
            >
              <Scale className="text-[#E8390E] mb-6" size={40} />
              <h3 className="font-display text-[24px] font-black uppercase text-white mb-4 tracking-tighter">{t("courtroomRules")}</h3>
              <p className="font-serif italic text-[18px] text-[#9A9590] leading-relaxed">"{t("courtroomTooltipDesc")}"</p>
              <button onClick={() => setShowTooltip(false)} className="mt-8 w-full py-4 bg-[#E8390E] font-display font-bold uppercase text-[12px] text-white">Understood</button>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}