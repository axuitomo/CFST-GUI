/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{vue,ts}"],
  theme: {
    extend: {
      boxShadow: {
        panel: "0 18px 48px rgba(15, 23, 42, 0.08)",
      },
      colors: {
        cf: "oklab(0.7163455 0.1016349 0.1370831)",
        primary: "oklab(0.5105731 0.0278995 -0.2283554)",
        primaryHover: "oklab(0.4567751 0.0262356 -0.2129681)",
      },
      fontFamily: {
        sans: ["Segoe UI", "PingFang SC", "Noto Sans SC", "sans-serif"],
      },
    },
  },
  plugins: [],
};
