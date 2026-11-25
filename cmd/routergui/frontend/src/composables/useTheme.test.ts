import { createTheme, THEME_STORAGE_KEY } from "./useTheme";

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial));
  return {
    values,
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => {
      values.set(key, value);
    },
  };
}

function systemScheme(dark: boolean) {
  let listener: ((event: { matches: boolean }) => void) | undefined;
  const query = {
    matches: dark,
    addEventListener: (_: string, callback: (event: { matches: boolean }) => void) => {
      listener = callback;
    },
    removeEventListener: () => {
      listener = undefined;
    },
  };
  return {
    matchMedia: () => query,
    change(next: boolean) {
      query.matches = next;
      listener?.({ matches: next });
    },
  };
}

describe("createTheme", () => {
  it("defaults to the light theme when nothing is stored", () => {
    const root = document.createElement("html");
    const theme = createTheme({
      storage: memoryStorage(),
      matchMedia: systemScheme(true).matchMedia,
      root,
    });
    expect(theme.preference.value).toBe("light");
    expect(theme.resolved.value).toBe("light");
    expect(root.dataset.theme).toBe("light");
  });

  it("restores a stored preference and ignores invalid values", () => {
    const dark = document.createElement("html");
    createTheme({
      storage: memoryStorage({ [THEME_STORAGE_KEY]: "dark" }),
      matchMedia: systemScheme(false).matchMedia,
      root: dark,
    });
    expect(dark.dataset.theme).toBe("dark");

    const invalid = document.createElement("html");
    const theme = createTheme({
      storage: memoryStorage({ [THEME_STORAGE_KEY]: "neon" }),
      matchMedia: systemScheme(true).matchMedia,
      root: invalid,
    });
    expect(theme.preference.value).toBe("light");
    expect(invalid.dataset.theme).toBe("light");
  });

  it("follows the system scheme live when the preference is system", () => {
    const root = document.createElement("html");
    const system = systemScheme(true);
    const theme = createTheme({
      storage: memoryStorage({ [THEME_STORAGE_KEY]: "system" }),
      matchMedia: system.matchMedia,
      root,
    });
    expect(theme.resolved.value).toBe("dark");
    expect(root.dataset.theme).toBe("dark");
    system.change(false);
    expect(theme.resolved.value).toBe("light");
    expect(root.dataset.theme).toBe("light");
  });

  it("persists only the preference and syncs the native window theme", () => {
    const root = document.createElement("html");
    const storage = memoryStorage();
    const runtime = {
      WindowSetLightTheme: vi.fn(),
      WindowSetDarkTheme: vi.fn(),
      WindowSetSystemDefaultTheme: vi.fn(),
    };
    const theme = createTheme({
      storage,
      matchMedia: systemScheme(false).matchMedia,
      root,
      runtime,
    });
    expect(runtime.WindowSetLightTheme).toHaveBeenCalledTimes(1);

    theme.setPreference("dark");
    expect(root.dataset.theme).toBe("dark");
    expect(runtime.WindowSetDarkTheme).toHaveBeenCalledTimes(1);
    expect([...storage.values.entries()]).toEqual([[THEME_STORAGE_KEY, "dark"]]);

    theme.setPreference("system");
    expect(runtime.WindowSetSystemDefaultTheme).toHaveBeenCalledTimes(1);
    expect(storage.values.get(THEME_STORAGE_KEY)).toBe("system");
  });

  it("toggles to the opposite of the resolved theme", () => {
    const root = document.createElement("html");
    const theme = createTheme({
      storage: memoryStorage(),
      matchMedia: systemScheme(false).matchMedia,
      root,
    });
    theme.toggle();
    expect(theme.preference.value).toBe("dark");
    theme.toggle();
    expect(theme.preference.value).toBe("light");
  });

  it("works without storage, media queries or a Wails runtime", () => {
    const root = document.createElement("html");
    const theme = createTheme({ storage: null, matchMedia: null, root, runtime: null });
    theme.setPreference("system");
    expect(theme.resolved.value).toBe("light");
    expect(root.dataset.theme).toBe("light");
  });
});
