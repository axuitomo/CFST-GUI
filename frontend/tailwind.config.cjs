/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./index.html", "./src/**/*.{vue,ts}"],
  theme: {
    extend: {
      boxShadow: {
        panel: "0 18px 48px rgba(15, 23, 42, 0.08)",
      },
      colors: {
        cf: "#f38020",
        primary: "#4f46e5",
        primaryHover: "#4338ca",
      },
      fontFamily: {
        sans: ["Segoe UI", "PingFang SC", "Noto Sans SC", "sans-serif"],
      },
    },
  },
  plugins: [],
};
