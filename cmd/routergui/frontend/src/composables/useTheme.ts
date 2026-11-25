import { computed, ref, type ComputedRef, type Ref } from "vue";

export type ThemePreference = "light" | "dark" | "system";
export type ResolvedTheme = "light" | "dark";

export const THEME_STORAGE_KEY = "router.theme";
const PREFERENCES: readonly ThemePreference[] = ["light", "dark", "system"];
const DARK_QUERY = "(prefers-color-scheme: dark)";
const SWITCHING_CLASS = "rt-theme-switching";

interface StorageLike {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
}

interface MediaQueryLike {
  matches: boolean;
  addEventListener(type: "change", listener: (event: { matches: boolean }) => void): void;
}

/** The Wails runtime's window-theme calls; only Windows implements them in Wails v2. */
export interface WindowThemeRuntime {
  WindowSetLightTheme?: () => void;
  WindowSetDarkTheme?: () => void;
  WindowSetSystemDefaultTheme?: () => void;
}

export interface ThemeOptions {
  storage?: StorageLike | null;
  matchMedia?: ((query: string) => MediaQueryLike) | null;
  root?: HTMLElement;
  runtime?: WindowThemeRuntime | null;
}

export interface ThemeController {
  preference: Ref<ThemePreference>;
  resolved: ComputedRef<ResolvedTheme>;
  setPreference(next: ThemePreference): void;
  toggle(): void;
}

function readPreference(storage: StorageLike | null | undefined): ThemePreference {
  try {
    const stored = storage?.getItem(THEME_STORAGE_KEY);
    return PREFERENCES.includes(stored as ThemePreference) ? (stored as ThemePreference) : "light";
  } catch {
    return "light";
  }
}

/**
 * Creates the appearance controller (DESIGN.md §1): light (白瓷) by default, dark (石墨) on
 * request, or the system scheme. It writes `data-theme` on the root element, stores only the
 * preference, and mirrors the theme onto the native window where the runtime supports it.
 */
export function createTheme(options: ThemeOptions = {}): ThemeController {
  const storage = options.storage;
  const root = options.root ?? document.documentElement;
  const runtime = options.runtime;
  let media: MediaQueryLike | null = null;
  try {
    media = options.matchMedia?.(DARK_QUERY) ?? null;
  } catch {
    media = null;
  }

  const preference = ref<ThemePreference>(readPreference(storage));
  const systemDark = ref(Boolean(media?.matches));
  media?.addEventListener("change", (event) => {
    systemDark.value = event.matches;
    apply();
  });

  const resolved = computed<ResolvedTheme>(() => {
    if (preference.value === "system") return systemDark.value ? "dark" : "light";
    return preference.value;
  });

  function syncWindow() {
    try {
      if (preference.value === "system") runtime?.WindowSetSystemDefaultTheme?.();
      else if (resolved.value === "dark") runtime?.WindowSetDarkTheme?.();
      else runtime?.WindowSetLightTheme?.();
    } catch {
      // The native title bar is cosmetic; the page theme is already applied.
    }
  }

  // DESIGN.md §5: no theme cross-fade. Transitions are suspended while the palette swaps, so
  // buttons and fields do not fade while cards change at once.
  function apply() {
    if (root.dataset.theme === resolved.value) return;
    root.classList.add(SWITCHING_CLASS);
    root.dataset.theme = resolved.value;
    void getComputedStyle(root).color;
    setTimeout(() => root.classList.remove(SWITCHING_CLASS), 0);
  }

  function setPreference(next: ThemePreference) {
    preference.value = next;
    try {
      storage?.setItem(THEME_STORAGE_KEY, next);
    } catch {
      // Storage is optional; the choice still applies to this window.
    }
    apply();
    syncWindow();
  }

  apply();
  syncWindow();

  return {
    preference,
    resolved,
    setPreference,
    toggle: () => setPreference(resolved.value === "dark" ? "light" : "dark"),
  };
}
