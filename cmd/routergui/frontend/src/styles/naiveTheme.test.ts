import { naiveOverrides } from "./naiveTheme";

describe("naiveOverrides", () => {
  it("maps design tokens onto Naive UI and skips tokens that are not available", () => {
    const tokens: Record<string, string> = {
      "--rt-surface": "#2a2e35",
      "--rt-field": "#262a30",
      "--rt-fg": "#eceef1",
      "--rt-fg-muted": "#a5acb6",
      "--rt-border": "#414750",
      "--rt-border-strong": "#4c535d",
      "--rt-accent-text": "#83befe",
      "--rt-danger": "#f0736a",
      "--rt-font-ui": '"Instrument Sans Variable", sans-serif',
    };
    const overrides = naiveOverrides((name) => tokens[name] ?? "");
    expect(overrides.common?.primaryColor).toBe("#83befe");
    expect(overrides.common?.errorColor).toBe("#f0736a");
    expect(overrides.common?.fontFamily).toBe('"Instrument Sans Variable", sans-serif');
    expect(overrides.Card?.color).toBe("#2a2e35");
    expect(overrides.Input?.color).toBe("#262a30");
    expect(overrides.Input?.border).toBe("1px solid #4c535d");
    expect(overrides.common).not.toHaveProperty("successColor");
  });
});
