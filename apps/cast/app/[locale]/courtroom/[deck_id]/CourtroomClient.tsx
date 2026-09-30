// app/[locale]/courtroom/[deck_id]/CourtroomClient.tsx
"use client";

import { useState, useEffect, useRef } from "react";
import Image from "next/image";
import { useRouter } from "@/i18n/routing";
import { useTranslations } from "next-intl";
import { motion, AnimatePresence } from "framer-motion";
import { Scale } from "lucide-react";

// Helper to generate Idempotency Keys (UUIDv4)
function generateUUID() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0, v = c == 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

export type SquadMember = {
  id: string;
  name: string;
  handle: string;
  avatar: string;
};

export type Question = {
  id: string;
  prompt: string;
};

interface CourtroomClientProps {
  deckId: string;
  squadName: string;
  members: SquadMember[];
  questions: Question[];
  sessionId: string;
}

export function CourtroomClient({ deckId, squadName, members, questions, sessionId }: CourtroomClientProps) {
  const t = useTranslations("Courtroom");
  const router = useRouter();
  
  const [introVisible, setIntroVisible] = useState(true);
  const [currentIndex, setCurrentIndex] = useState(0);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);

  // Pre-load audio for zero-latency playback
  const tickSound = useRef<HTMLAudioElement | null>(null);

  useEffect(() => {
    if (typeof window !== "undefined") {
      tickSound.current = new Audio("/sounds/tick.mp3");
      tickSound.current.volume = 0.5; // Subtle, not deafening
    }
    const timer = setTimeout(() => setIntroVisible(false), 2500);
    return () => clearTimeout(timer);
  }, []);

  const currentQuestion = questions[currentIndex];
  const isFinalQuestion = currentIndex === questions.length - 1;

  // 1. INTRO CINEMATIC EFFECT
  useEffect(() => {
    const timer = setTimeout(() => setIntroVisible(false), 2500);
    return () => clearTimeout(timer);
  }, []);

  // ── ADVANCED VOTING LOGIC ──
  const handleVote = async (userId: string) => {
    if (selectedUserId) return; // Prevent double taps locally

    // 1. SENSORY FEEDBACK
    if (typeof navigator !== "undefined" && navigator.vibrate) navigator.vibrate(40);
    if (tickSound.current) {
      tickSound.current.currentTime = 0; // Reset sound to start
      tickSound.current.play().catch(() => {}); // Catch browser auto-play blocks
    }

    setSelectedUserId(userId);

    // 2. IDEMPOTENCY & OFFLINE QUEUE
    const idempotencyKey = generateUUID();
    const votePayload = {
      deckId,
      questionId: currentQuestion.id,
      targetUserId: userId,
      timestamp: new Date().toISOString(),
    };

    try {
      // In production: ConnectRPC Call
      // await rpc.castVote(votePayload, { headers: { "Idempotency-Key": idempotencyKey } });
      
      console.log("Vote cast successfully", votePayload);
    } catch {
      // 3. THE WEST AFRICAN EDGE CASE (Network Drop)
      console.warn("Network failed. Queuing vote locally.");
      const queuedVotes = JSON.parse(localStorage.getItem("cast_offline_votes") || "[]");
      queuedVotes.push({ ...votePayload, idempotencyKey });
      localStorage.setItem("cast_offline_votes", JSON.stringify(queuedVotes));
      
      // Note: A background Go Worker/Service Worker will pick this up when online
    }

    // 4. CINEMATIC TRANSITION
    setTimeout(() => {
      if (isFinalQuestion) {
        router.push(`/courtroom/results/${sessionId}`);
      } else {
        setCurrentIndex(prev => prev + 1);
        setSelectedUserId(null); 
      }
    }, 400);
  };

  const handleSkip = () => {
    if (selectedUserId) return;
    
    // The Product Bible explicitly states: "Abstentions are excluded from percentage calculations but logged for Hermes squad lore."
    try {
      // await rpc.logAbstention({ deckId, questionId: currentQuestion.id });
    } catch {
      // Queue offline if needed
    }

    if (isFinalQuestion) {
      router.push(`/courtroom/results/${sessionId}`);
    } else {
      setCurrentIndex(prev => prev + 1);
    }
  };

  return (
    <div className="w-full h-full max-w-md mx-auto relative flex flex-col">
      
      {/* ── INTRO OVERLAY ── */}
      <AnimatePresence>
        {introVisible && (
          <motion.div 
            initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0, scale: 1.05, filter: "blur(10px)" }}
            transition={{ duration: 0.6, ease: [0.76, 0, 0.24, 1] }}
            className="absolute inset-0 z-50 bg-[#080808] flex flex-col items-center justify-center p-6 text-center"
          >
            <Scale className="w-12 h-12 text-[#E8390E] mb-6 animate-pulse" />
            <h1 className="font-display text-[40px] font-extrabold text-[#F0EDE8] leading-[0.9] tracking-[-0.03em] uppercase mb-4">
              {t("inSession")}
            </h1>
            <p className="font-mono text-[10px] text-[#9A9590] uppercase tracking-widest leading-relaxed">
              {t("anonymousWarning")}
            </p>
          </motion.div>
        )}
      </AnimatePresence>

      {/* ── VOTING UI ── */}
      <div className="flex-1 flex flex-col p-5 pt-12 relative z-10">
        
        {/* Header */}
        <div className="text-center mb-10">
          <div className="font-mono text-[10px] text-text-tertiary uppercase tracking-widest mb-4">
            {t("pageTitle")} · {squadName}
          </div>
          
          <AnimatePresence mode="wait">
            <motion.h2 
              key={currentQuestion.id}
              initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }}
              transition={{ duration: 0.3 }}
              className="font-display text-[28px] sm:text-[32px] font-extrabold text-[#F0EDE8] leading-[1.05] tracking-[-0.02em] uppercase"
            >
              {currentQuestion.prompt}
            </motion.h2>
          </AnimatePresence>
        </div>

        {/* Face Grid (Tinder-style) */}
        <div className="flex-1 grid grid-cols-2 gap-3 pb-8">
          {members.map((member) => {
            const isSelected = selectedUserId === member.id;
            const isNotSelected = selectedUserId !== null && selectedUserId !== member.id;

            return (
              <button
                key={member.id}
                onClick={() => handleVote(member.id)}
                disabled={selectedUserId !== null}
                className={`relative w-full h-full rounded-module overflow-hidden border-2 transition-all duration-300 flex flex-col justify-end p-4 text-start
                  ${isSelected ? "border-[#E8390E] scale-95 shadow-[0_0_20px_rgba(232,57,14,0.3)] z-20" : "border-white/5 bg-[#111111] hover:border-white/20 active:scale-95"}
                  ${isNotSelected ? "opacity-30 grayscale scale-95" : "opacity-100"}
                `}
              >
                {/* Background Image */}
                <div className="absolute inset-0">
                  <Image 
                    src={member.avatar} 
                    alt={member.name} 
                    fill 
                    className="object-cover opacity-60 saturate-50 mix-blend-luminosity"
                  />
                  <div className={`absolute inset-0 bg-linear-to-t transition-colors duration-300 ${isSelected ? "from-[#E8390E]/60 via-[#080808]/80" : "from-[#080808] via-[#080808]/50"} to-transparent`} />
                </div>

                <div className="relative z-10 flex flex-col">
                  <span className="font-display text-[22px] font-extrabold text-[#F0EDE8] leading-none mb-1">
                    {member.name}
                  </span>
                  <span className={`font-mono text-[9px] uppercase tracking-widest ${isSelected ? "text-[#F0EDE8]" : "text-[#9A9590]"}`}>
                    {member.handle}
                  </span>
                </div>
              </button>
            );
          })}
        </div>

        {/* Footer: Progress & Skip */}
        <div className="mt-auto flex flex-col items-center gap-6 pb-6">
          <button 
            onClick={handleSkip}
            disabled={selectedUserId !== null}
            className="font-mono text-[10px] text-text-tertiary hover:text-[#9A9590] uppercase tracking-widest transition-colors disabled:opacity-0"
          >
            {t("skip")}
          </button>

          {/* Progress Indicator (Pills) */}
          <div className="w-full">
            <div className="flex justify-between items-end mb-3">
              <span className="font-mono text-[9px] text-[#E8390E] font-bold uppercase tracking-widest">
                {isFinalQuestion ? t("finalQuestion") : t("tapToCast")}
              </span>
              <span className="font-mono text-[9px] text-text-tertiary uppercase tracking-widest">
                {t("progress", { current: currentIndex + 1, total: questions.length })}
              </span>
            </div>
            
            {/* Dots */}
            <div className="flex gap-1.5 justify-center">
              {questions.map((_, idx) => (
                <div 
                  key={idx} 
                  className={`h-1 rounded-full flex-1 transition-all duration-300 ${
                    idx === currentIndex 
                      ? "bg-[#E8390E]" 
                      : idx < currentIndex 
                        ? "bg-[#E8390E]/30" 
                        : "bg-white/10"
                  }`}
                />
              ))}
            </div>
          </div>
        </div>

      </div>
    </div>
  );
}