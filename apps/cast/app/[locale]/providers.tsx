// app/[locale]/providers.tsx
"use client";

import { QueryClientProvider } from "@tanstack/react-query";
import { TransportProvider } from "@connectrpc/connect-query";
import { queryClient, transport } from "@/lib/rpc/client";

export function Providers({ children }: { children: React.ReactNode }) {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    </TransportProvider>
  );
}