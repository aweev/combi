// components/BottomNav.tsx
"use client";

import { useTranslations } from "next-intl";
import { Link, usePathname } from "@/i18n/routing";
import { Zap, Users, Scale, Trophy, LockKeyhole } from "lucide-react";

export function BottomNav() {
  const t = useTranslations("Navigation");
  const pathname = usePathname(); // This automatically strips the /en or /fr prefix!

  const tabs = [
    { name: t("home"), path: "/home", icon: Zap },
    { name: t("squads"), path: "/squads", icon: Users },
    { name: t("court"), path: "/decks", icon: Scale }, // You mentioned you flattened this to /decks!
    { name: t("arena"), path: "/arena", icon: Trophy },
    { name: t("vault"), path: "/vault", icon: LockKeyhole },
  ];

  return (
    <nav className="fixed bottom-0 inset-x-0 bg-[#080808]/90 backdrop-blur-xl border-t border-white/10 pb-safe z-50">
      <div className="max-w-md mx-auto flex justify-between px-2 pt-3 pb-4">
        {tabs.map((tab) => {
          // Check if active (handles sub-routes like /decks/123)
          const isActive = pathname.startsWith(tab.path);

          return (
            <Link
              key={tab.path}
              href={tab.path}
              className="flex-1 flex flex-col items-center gap-1.5 transition-colors duration-200 active:scale-95"
            >
              <tab.icon
                strokeWidth={isActive ? 2.5 : 2}
                className={`w-6 h-6 ${
                  isActive ? "text-[#E8390E]" : "text-text-tertiary"
                }`}
              />
              <span
                className={`font-mono text-[9px] tracking-widest uppercase ${
                  isActive ? "text-[#E8390E] font-bold" : "text-text-tertiary font-medium"
                }`}
              >
                {tab.name}
              </span>
            </Link>
          );
        })}
      </div>
    </nav>
  );
}