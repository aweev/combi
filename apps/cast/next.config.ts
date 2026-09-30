// apps/cast/next.config.ts
import type { NextConfig } from 'next'; // Add this import
import createNextIntlPlugin from 'next-intl/plugin';
import withPWAInit from "@ducanh2912/next-pwa";

const withNextIntl = createNextIntlPlugin('./i18n/request.ts');

const withPWA = withPWAInit({
  dest: "public",
  disable: process.env.NODE_ENV === "development",
  register: true,
  /* Cast requires a seamless offline experience for the MoMo holding cell */
  cacheOnFrontEndNav: true,
  aggressiveFrontEndNavCaching: true,
});

// Remove the JSDoc comment and use the TypeScript type instead:
const nextConfig: NextConfig = {
  images: {
    remotePatterns: [
      {
        protocol: 'https',
        hostname: 'i.pravatar.cc',
        pathname: '/**',
      },
      {
        protocol: 'https',
        hostname: 'images.unsplash.com', 
        pathname: '/**',
      },
    ],
  },
  reactStrictMode: true,
  // Add backend image domains here later
};

export default withPWA(withNextIntl(nextConfig));