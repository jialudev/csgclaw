/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  // The application owns its reset and component styles.
  corePlugins: { preflight: false },
  theme: { extend: {} },
  plugins: [],
};
