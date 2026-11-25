import tokensCss from "./tokens.css?raw";

// DESIGN.md: every colour, size, spacing and radius in a component resolves to a token.
const defined = new Set([...tokensCss.matchAll(/(--rt-[\w-]+)\s*:/g)].map((match) => match[1]));

const raw = import.meta.glob(["../**/*.vue", "../**/*.css", "!../styles/tokens.css"], {
  query: "?raw",
  import: "default",
  eager: true,
}) as Record<string, string>;

function styleOf(path: string, text: string): string {
  if (path.endsWith(".css")) return text;
  return [...text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((match) => match[1]).join("\n");
}

const files = Object.entries(raw).map(([path, text]) => ({
  name: path.replace(/^\.\.\//, ""),
  style: styleOf(path, text),
  all: text,
}));

it("finds the component sources", () => {
  expect(files.length).toBeGreaterThan(10);
});

it.each(files.map((file) => [file.name, file] as const))(
  "%s only references defined tokens",
  (_, file) => {
    const used = [...file.all.matchAll(/var\((--rt-[\w-]+)/g)].map((match) => match[1]);
    expect(used.filter((name) => !defined.has(name))).toEqual([]);
  },
);

it.each(files.map((file) => [file.name, file] as const))(
  "%s has no literal colours in its styles",
  (_, file) => {
    const literals = file.style.match(/#[0-9a-f]{3,8}\b|\brgba?\(|\bhsla?\(/gi) ?? [];
    expect(literals).toEqual([]);
  },
);

it.each(files.map((file) => [file.name, file] as const))(
  "%s has no raw pixel sizes in its styles",
  (_, file) => {
    // Media/container-query breakpoints and 1px (hairlines, visually hidden inputs) are the only exceptions.
    const withoutMedia = file.style.replace(/@(media|container)[^{]*\{/g, "");
    const literals = (withoutMedia.match(/-?\b\d+(\.\d+)?px\b/g) ?? []).filter(
      (value) => value !== "1px" && value !== "-1px",
    );
    expect(literals).toEqual([]);
  },
);
