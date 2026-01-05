import { defineConfig } from "vite";

import tailwindcss from "@tailwindcss/vite";
import path from "path";
import react from "@vitejs/plugin-react";
import { analyzer } from "vite-bundle-analyzer";

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss() /*, analyzer()*/],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
