// app/[locale]/courtroom/execution/ExecutionFooter.tsx
"use client";

import { motion } from "framer-motion";
import { Share, Zap } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Link } from "@/i18n/routing";

export function ExecutionFooter() {
  const tCom = useTranslations("Common");
  const tExec = useTranslations("Execution");
  const locale = useLocale();

  const handleShare = async () => {
    // 1. Get the current domain dynamically (e.g., http://localhost:3000 or https://yourdomain.com)
    const baseUrl = typeof window !== "undefined" ? window.location.origin : "";
    
    // 2. Construct the localized URL! 
    // Example: https://yourdomain.com/fr/vote/session-123
    const shareUrl = `${baseUrl}/${locale}/vote/session-123`; 

    if (navigator.share) {
      try {
        await navigator.share({
          title: tExec("shareTitle"),
          text: tExec("shareText"),
          url: shareUrl, 
        });
      } catch (error) {
        console.log("Error sharing:", error);
      }
    } else {
      navigator.clipboard.writeText(shareUrl);
      alert(tExec("copiedSuccess"));
    }
  };

  return (
    <motion.div 
      className="fixed bottom-0 left-0 right-0 p-5 z-50 max-w-md mx-auto"
      initial={{ opacity: 0, y: 100 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ delay: 0.8, duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
    >
      {/* Dark gradient to mask scroll area cleanly */}
      <div className="absolute inset-0 bg-linear-to-t from-[#080808] via-[#080808]/95 to-transparent -z-10 pointer-events-none h-[150%]" />
      
      <div className="flex gap-3 relative z-10">
        <button onClick={handleShare} className="flex-1 bg-transparent border border-white/20 text-[#F0EDE8] rounded-[6px] h-14 font-display font-bold text-[14px] tracking-[0.02em] uppercase flex items-center justify-center gap-2 hover:bg-white/5 transition-colors backdrop-blur-md">
          <Share className="w-4 h-4" /> {tCom("shareToStatus")}
        </button>
        <Link href="/decks" className="flex-1 bg-[#E8390E] text-white rounded-[6px] h-14 font-display font-bold text-[14px] tracking-[0.02em] uppercase flex items-center justify-center gap-2 hover:bg-[#C43209] transition-all shadow-[0_0_20px_rgba(232,57,14,0.3)] hover:shadow-[0_0_30px_rgba(232,57,14,0.5)] active:scale-95">
          <Zap className="w-5 h-5 fill-white" /> {tCom("castAgain")}
        </Link>
      </div>
    </motion.div>
  );
}