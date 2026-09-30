import { createConnectTransport } from "@connectrpc/connect-web";
import { QueryClient } from "@tanstack/react-query";
import type { Interceptor } from "@connectrpc/connect";

// 1. Write the client-side Interceptor with explicit type checking
const clientHeadersInterceptor: Interceptor = (next) => (req) => {
  // Read token from local storage safely during client-side execution
  const token = typeof window !== "undefined" ? localStorage.getItem("cast_jwt") : null;
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }

  // Detect PWA language locale and attach context header [1.2.2]
  if (typeof window !== "undefined") {
    const locale = document.documentElement.lang || "en";
    req.header.set("Accept-Language", locale); // Instructs Go backend and Hermes AI [p. 10]
  }

  return next(req);
};

// 2. Instantiate the transport
export const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080",
  useBinaryFormat: true, // Optimizes payload overhead on low data budgets [p. 21]
  interceptors: [clientHeadersInterceptor],
});

// 3. Setup the TanStack Query Client
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 1000 * 10, // 10 seconds
      retry: 2,
      refetchOnWindowFocus: true, // Auto-refreshes when user returns to PWA [p. 4]
    },
  },
});