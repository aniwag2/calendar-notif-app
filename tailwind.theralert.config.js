/** @type {import('tailwindcss').Config} */
// Config for the Go rewrite. The old Next.js app keeps using tailwind.config.js.
module.exports = {
  content: ["./web/templates/**/*.html"],
  theme: {
    extend: {
      colors: {
        // category colors used across the calendar
        therapy: "#2563eb", // blue
        activity: "#16a34a", // green
        appointment: "#db2777", // pink
      },
    },
  },
  plugins: [require("@tailwindcss/forms")],
};
