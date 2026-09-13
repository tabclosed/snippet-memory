import { PrismLight as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";

// react-syntax-highlighter ships a "light build" (PrismLight) that only
// includes the languages you explicitly register below, instead of every
// language Prism supports — keeps the JS bundle small. Register whatever
// languages you expect snippets to use; add more here as needed.
import go from "react-syntax-highlighter/dist/esm/languages/prism/go";
import typescript from "react-syntax-highlighter/dist/esm/languages/prism/typescript";
import javascript from "react-syntax-highlighter/dist/esm/languages/prism/javascript";
import jsx from "react-syntax-highlighter/dist/esm/languages/prism/jsx";
import tsx from "react-syntax-highlighter/dist/esm/languages/prism/tsx";
import python from "react-syntax-highlighter/dist/esm/languages/prism/python";
import bash from "react-syntax-highlighter/dist/esm/languages/prism/bash";
import json from "react-syntax-highlighter/dist/esm/languages/prism/json";
import css from "react-syntax-highlighter/dist/esm/languages/prism/css";
import markup from "react-syntax-highlighter/dist/esm/languages/prism/markup"; // HTML/XML
import sql from "react-syntax-highlighter/dist/esm/languages/prism/sql";
import yaml from "react-syntax-highlighter/dist/esm/languages/prism/yaml";
import powershell from "react-syntax-highlighter/dist/esm/languages/prism/powershell";
import markdown from "react-syntax-highlighter/dist/esm/languages/prism/markdown";

SyntaxHighlighter.registerLanguage("go", go);
SyntaxHighlighter.registerLanguage("typescript", typescript);
SyntaxHighlighter.registerLanguage("javascript", javascript);
SyntaxHighlighter.registerLanguage("jsx", jsx);
SyntaxHighlighter.registerLanguage("tsx", tsx);
SyntaxHighlighter.registerLanguage("python", python);
SyntaxHighlighter.registerLanguage("bash", bash);
SyntaxHighlighter.registerLanguage("json", json);
SyntaxHighlighter.registerLanguage("css", css);
SyntaxHighlighter.registerLanguage("html", markup);
SyntaxHighlighter.registerLanguage("sql", sql);
SyntaxHighlighter.registerLanguage("yaml", yaml);
SyntaxHighlighter.registerLanguage("powershell", powershell);
SyntaxHighlighter.registerLanguage("markdown", markdown);

// Snippets store language as a free-typed string (e.g. "go", "TypeScript",
// "py"). This maps common variants/aliases to the exact names registered
// above, so casing or shorthand still highlights correctly.
const LANGUAGE_ALIASES: Record<string, string> = {
  go: "go",
  golang: "go",
  typescript: "typescript",
  ts: "typescript",
  javascript: "javascript",
  js: "javascript",
  jsx: "jsx",
  tsx: "tsx",
  python: "python",
  py: "python",
  bash: "bash",
  shell: "bash",
  sh: "bash",
  json: "json",
  css: "css",
  html: "html",
  sql: "sql",
  yaml: "yaml",
  yml: "yaml",
  powershell: "powershell",
  pwsh: "powershell",
  ps1: "powershell",
  markdown: "markdown",
  md: "markdown",
};

function resolveLanguage(language: string): string {
  const key = language.trim().toLowerCase();
  return LANGUAGE_ALIASES[key] ?? "text";
}

// CSS's native `text-overflow: ellipsis` cuts off wherever the pixel
// width runs out, mid-word as often as not ("frag" + "..."). Code is
// monospace, so instead we truncate by character count ourselves and
// back up to the previous space — guarantees the cutoff always reads as
// "text ..." with a space before the dots, never "text...".
function truncateAtWord(line: string, maxChars: number): string {
  if (line.length <= maxChars) return line;
  const cut = line.slice(0, maxChars);
  const lastSpace = cut.lastIndexOf(" ");
  const base = (lastSpace > 0 ? cut.slice(0, lastSpace) : cut).trimEnd();
  return `${base} ...`;
}

// Approximate max characters per line before truncating, for a card at
// typical desktop widths. It's a fixed guess rather than a measurement
// of the actual rendered width, so very narrow viewports may still wrap
// via the overflow:hidden safety net below — but it keeps the ellipsis
// logic simple and avoids a layout-measuring effect on every render.
const MAX_BLOCK_LINE_CHARS = 78;
const MAX_INLINE_LINE_CHARS = 55;

interface CodeBlockProps {
  code: string;
  language: string;
  /** "block" (default) is the full code block used on the detail page.
   *  "inline" is a compact single-line style for the list preview. */
  variant?: "block" | "inline";
  /** When true, drops the rounded corners so the block can be stretched
   *  edge-to-edge inside a container (e.g. the continuous category view),
   *  instead of sitting inset within the card's padding. */
  fullBleed?: boolean;
  /** When true, long lines scroll horizontally instead of being cut off
   *  with "...". Used on the detail page, where you want to actually read
   *  every character; the main-screen card previews keep the default
   *  truncating behavior since they're just previews. */
  scrollable?: boolean;
}

export function CodeBlock({
  code,
  language,
  variant = "block",
  fullBleed = false,
  scrollable = false,
}: CodeBlockProps) {
  const isInline = variant === "inline";
  const truncateLines = !isInline && !scrollable;

  // Build the text actually passed to the highlighter: for the
  // truncating cases, cut each line at the last word boundary within
  // the character budget rather than letting CSS clip mid-word.
  let displayCode = code;
  if (isInline) {
    displayCode = truncateAtWord(code, MAX_INLINE_LINE_CHARS);
  } else if (truncateLines) {
    displayCode = code
      .split("\n")
      .map((line) => truncateAtWord(line, MAX_BLOCK_LINE_CHARS))
      .join("\n");
  }

  return (
    <SyntaxHighlighter
      language={resolveLanguage(language)}
      style={vscDarkPlus}
      // Lines are pre-truncated to fit above (guaranteeing a space before
      // the "..."), but that's based on a fixed character-count guess,
      // not the card's actual rendered width. text-overflow: ellipsis
      // here is a safety net for when that guess runs a little wide —
      // it always shows *some* truncation indicator, even if in that
      // rare case it can't guarantee the space-before-dots styling.
      wrapLines={truncateLines}
      lineProps={
        truncateLines
          ? {
              style: {
                display: "block",
                // A genuinely empty line (blank lines in the snippet)
                // produces a span with no content — without an explicit
                // height, browsers collapse it to nothing, so blank
                // lines silently vanished from the card preview. This
                // keeps every line's height consistent whether it has
                // content or not.
                minHeight: "1.4em",
                whiteSpace: "nowrap",
                overflow: "hidden",
                textOverflow: "ellipsis",
              },
            }
          : undefined
      }
      customStyle={
        isInline
          ? {
              margin: 0,
              padding: "0.25rem 0.6rem",
              borderRadius: "5px",
              fontSize: "0.8rem",
              minWidth: 0,
              maxWidth: "100%",
              whiteSpace: "pre",
              overflow: "hidden",
              textOverflow: "ellipsis",
              background: "#000000",
            }
          : {
              margin: 0,
              borderRadius: fullBleed ? 0 : "8px",
              fontSize: "0.9rem",
              background: "#000000",
              overflowX: scrollable ? "auto" : "hidden",
            }
      }
    >
      {displayCode}
    </SyntaxHighlighter>
  );
}
