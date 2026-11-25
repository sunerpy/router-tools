import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";

export default defineConfig({
  plugins: [vue()],
  server: { host: "127.0.0.1", port: 5173 },
  test: {
    environment: "jsdom",
    globals: true,
    include: ["src/**/*.test.ts"],
    setupFiles: ["src/testSetup.ts"],
    css: true,
  },
});
