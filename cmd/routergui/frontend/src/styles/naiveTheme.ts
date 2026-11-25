import type { GlobalThemeOverrides } from "naive-ui";

type TokenReader = (name: string) => string;

/** Reads a design token from the document root after `data-theme` has been applied. */
export function documentTokens(root: HTMLElement = document.documentElement): TokenReader {
  const style = getComputedStyle(root);
  return (name) => style.getPropertyValue(name).trim();
}

function pick(entries: Record<string, string>): Record<string, string> {
  return Object.fromEntries(
    Object.entries(entries).filter(([, value]) => value && !value.includes("undefined")),
  );
}

/**
 * Maps the design tokens onto the Naive UI components the app uses (modal, card, input) so the
 * dialogs follow the active theme. Tokens that cannot be read (for example in tests without the
 * stylesheet) are left to Naive's defaults instead of being set to empty strings.
 */
export function naiveOverrides(token: TokenReader): GlobalThemeOverrides {
  const accent = token("--rt-accent-text");
  const borderStrong = token("--rt-border-strong");
  return {
    common: pick({
      fontFamily: token("--rt-font-ui"),
      fontFamilyMono: token("--rt-font-mono"),
      primaryColor: accent,
      primaryColorHover: accent,
      primaryColorPressed: accent,
      primaryColorSuppl: accent,
      infoColor: accent,
      errorColor: token("--rt-danger"),
      textColorBase: token("--rt-fg"),
      textColor1: token("--rt-fg"),
      textColor2: token("--rt-fg"),
      textColor3: token("--rt-fg-muted"),
      placeholderColor: token("--rt-fg-muted"),
      borderColor: borderStrong,
      dividerColor: token("--rt-border"),
      cardColor: token("--rt-surface"),
      modalColor: token("--rt-surface"),
      popoverColor: token("--rt-surface"),
      inputColor: token("--rt-field"),
      borderRadius: "6px",
    }),
    Card: pick({
      color: token("--rt-surface"),
      colorModal: token("--rt-surface"),
      borderColor: token("--rt-border"),
      titleTextColor: token("--rt-fg"),
      textColor: token("--rt-fg"),
      borderRadius: "10px",
    }),
    Input: pick({
      color: token("--rt-field"),
      colorFocus: token("--rt-field"),
      textColor: token("--rt-fg"),
      placeholderColor: token("--rt-fg-muted"),
      border: borderStrong ? `1px solid ${borderStrong}` : "",
      borderHover: accent ? `1px solid ${accent}` : "",
      borderFocus: accent ? `1px solid ${accent}` : "",
      heightMedium: "40px",
      borderRadius: "6px",
    }),
  };
}
