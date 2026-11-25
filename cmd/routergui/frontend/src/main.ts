import { createApp } from "vue";
import App from "./App.vue";
import { router } from "./router";
import { createTheme } from "./composables/useTheme";
import "./styles/base.css";

function localStorageOrNull(): Storage | null {
  try {
    return window.localStorage;
  } catch {
    return null;
  }
}

// Apply the appearance before mounting so a stored dark preference does not flash light.
const theme = createTheme({
  storage: localStorageOrNull(),
  matchMedia: typeof window.matchMedia === "function" ? (query) => window.matchMedia(query) : null,
  runtime: window.runtime ?? null,
});

createApp(App).provide("theme", theme).use(router).mount("#app");
