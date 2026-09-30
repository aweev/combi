import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        cast: {
          black: "#080808",
          white: "#F5F3EE",
          red: "#E8390E",
          "red-dark": "#B82C08",
          gold: "#C9A84C",
          s1: "#111111",
          s2: "#1A1A1A",
          s3: "#222222",
        },
      },
      fontFamily: {
        display: ["Syne", "sans-serif"],
        mono: ["DM Mono", "monospace"],
        serif: ["Instrument Serif", "serif"],
      },
    },
  },
  plugins: [],
};

export default config;
