import css from "./tokens.css?raw";

function themeBlock(theme: "light" | "dark"): Map<string, string> {
  const match = css.match(new RegExp(`\\[data-theme="${theme}"\\][^{]*\\{([^}]*)\\}`));
  if (!match) throw new Error(`no [data-theme="${theme}"] block in tokens.css`);
  const values = new Map<string, string>();
  for (const [, name, value] of match[1].matchAll(/(--rt-[\w-]+)\s*:\s*([^;]+);/g)) {
    values.set(name, value.trim());
  }
  return values;
}

function luminance(hex: string): number {
  const channels = hex
    .replace("#", "")
    .match(/../g)!
    .map((part) => parseInt(part, 16) / 255)
    .map((c) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2];
}

function contrast(a: string, b: string): number {
  const [high, low] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (high + 0.05) / (low + 0.05);
}

const TEXT_PAIRS: Array<[string, string]> = [
  ...[
    "canvas",
    "surface",
    "sidebar",
    "inset",
    "inset-strong",
    "field",
    "nav-active",
    "accent-soft",
  ].map((bg) => ["fg", bg] as [string, string]),
  ...["canvas", "surface", "sidebar", "inset", "inset-strong", "field", "nav-active"].map(
    (bg) => ["fg-muted", bg] as [string, string],
  ),
  ["accent-text", "canvas"],
  ["accent-text", "surface"],
  ["accent-text", "sidebar"],
  ["accent-text", "accent-soft"],
  ["ok-text", "ok-soft"],
  ["ok-text", "surface"],
  ["warning-text", "warning-soft"],
  ["warning-text", "surface"],
  ["danger-text", "danger-soft"],
  ["danger-text", "surface"],
  ["fg", "danger-soft"],
  ["fg", "warning-soft"],
  ["primary-fg", "primary"],
  ["danger-fg", "danger"],
  ["highlight-fg", "highlight-from"],
  ["highlight-fg", "highlight-to"],
  ["highlight-muted", "highlight-from"],
  ["highlight-muted", "highlight-to"],
];

const NON_TEXT_PAIRS: Array<[string, string]> = [
  ["accent-text", "nav-active"],
  ["toggle-on", "surface"],
  ["ok", "surface"],
  ["ok", "sidebar"],
  ["warning", "surface"],
  ["danger", "surface"],
];

describe.each(["light", "dark"] as const)("%s theme tokens", (theme) => {
  const tokens = themeBlock(theme);
  const color = (name: string) => {
    const value = tokens.get(`--rt-${name}`);
    if (!value || !/^#[0-9a-f]{6}$/i.test(value)) {
      throw new Error(`--rt-${name} in the ${theme} theme must be a #rrggbb literal, got ${value}`);
    }
    return value;
  };

  it.each(TEXT_PAIRS)("text %s on %s is at least 4.5:1", (fg, bg) => {
    expect(contrast(color(fg), color(bg))).toBeGreaterThanOrEqual(4.5);
  });

  it.each(NON_TEXT_PAIRS)("indicator %s on %s is at least 3:1", (fg, bg) => {
    expect(contrast(color(fg), color(bg))).toBeGreaterThanOrEqual(3);
  });
});

it("defines the same tokens in both themes", () => {
  expect([...themeBlock("dark").keys()].sort()).toEqual([...themeBlock("light").keys()].sort());
});
