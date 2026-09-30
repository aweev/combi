// components/TopBar.tsx
"use client";

import { useLocale } from "next-intl";
import { Bell } from "lucide-react";
import { Link } from "@/i18n/routing";

export function TopBar() {
  const locale = useLocale();
  
  // Format the date dynamically based on the user's locale
  const today = new Intl.DateTimeFormat(locale, {
    weekday: "short",
    day: "numeric",
    month: "short"
  }).format(new Date());

  return (
    <header className="sticky top-0 inset-x-0 bg-[#080808]/90 backdrop-blur-md border-b border-white/10 z-50">
      <div className="max-w-md mx-auto px-5 h-14 flex items-center justify-between">
        
        {/* LOGO */}
        <Link href="/home" className="font-display text-[20px] font-extrabold tracking-[-0.04em] text-[#F0EDE8]">
          C<span className="text-[#E8390E]">A</span>ST
        </Link>
        
        {/* DYNAMIC DATE */}
        <div className="font-mono text-[11px] text-[#9A9590] uppercase tracking-widest">
          {today}
        </div>
        
        {/* NOTIFICATION BELL */}
        <Link href="/notifications" className="relative w-8 h-8 rounded-full border border-white/10 flex items-center justify-center text-[#F0EDE8]/60 hover:text-[#F0EDE8] hover:bg-white/5 transition-colors">
          <Bell className="w-4 h-4" />
          {/* Red pulse dot for unread notifications */}
          <span className="absolute top-1.5 inset-e-1.5 w-2 h-2 rounded-full bg-[#E8390E] border-2 border-[#080808] animate-pulse" />
        </Link>

      </div>
    </header>
  );
}