// app/[locale]/(orbit)/layout.tsx
import { TopBar } from "@/components/TopBar";
import { BottomNav } from "@/components/BottomNav";

export default function OrbitLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col min-h-dvh bg-[#080808] text-[#F0EDE8] overflow-hidden selection:bg-[#E8390E] selection:text-white">
      {/* Global Top Bar */}
      <TopBar />
      
      {/* Main Content Area (Scrollable) */}
      <main className="flex-1 overflow-y-auto [&::-webkit-scrollbar]:hidden pb-24">
        {children}
      </main>

      {/* Global Bottom Navigation */}
      <BottomNav />
    </div>
  );
}