// proxy.ts
import { type NextRequest } from "next/server";
import createMiddleware from "next-intl/middleware";
import { routing } from "./i18n/routing";

// Initialize the next-intl middleware
const intlMiddleware = createMiddleware(routing);

export default async function middleware(request: NextRequest) {
  // Step 0: Handle Language Routing
  const response = intlMiddleware(request);
  
  // (In Step 3, we will add Supabase Auth token validation here to protect /app/* routes)

  return response;
}

export const config = {
  // Match all paths except API routes, static assets, manifests, and icons
  matcher: ["/((?!api|_next|_vercel|sounds|icons|.*\\..*).*)"],
};