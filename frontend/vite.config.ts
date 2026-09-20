/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Geliştirmede /api isteklerini Go API'ye yönlendir: tarayıcı açısından frontend ve API
// AYNI origin'de (localhost:5173) → CORS gerekmez. Prod'da aynı işi NGINX yapar (Faz 11).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": { target: process.env.API_URL ?? "http://localhost:8080", changeOrigin: false },
    },
  },
  test: {
    environment: "jsdom",
    // YALNIZ src/: vitest'in varsayılan deseni ("**/*.{test,spec}.*") e2e/*.spec.ts
    // dosyalarını da toplardı. Onlar Playwright'in testleri — jsdom'da koşmaya
    // çalışırlarsa "test is not defined" ile patlarlar ve `npm test` sebepsiz kırmızı olur.
    include: ["src/**/*.test.{ts,tsx}"],
  },
});
