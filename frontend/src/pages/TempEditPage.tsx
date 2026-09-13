import { useEffect, useState } from "react";
import { Link, useLocation, useParams } from "react-router-dom";
import ReactSimpleCodeEditor from "react-simple-code-editor";
import Prism from "prismjs";
import "prismjs/components/prism-markup";
import "prismjs/components/prism-css";
import "prismjs/components/prism-javascript";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-go";
import "prismjs/components/prism-json";
import "prismjs/components/prism-python";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-yaml";
import "prismjs/components/prism-powershell";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-jsx";
import "prismjs/components/prism-tsx";
import "prismjs/components/prism-markdown";
import "prismjs/themes/prism-tomorrow.css";
import { fetchSnippet } from "../api";
import { copyToClipboard } from "../utils/clipboard";
import type { Snippet } from "../types";

// Same alias table as CodeBlock.tsx, but mapping to Prism's own language
// keys (mostly identical, except "html"/"shell" etc., which Prism only
// knows by their canonical name).
const PRISM_LANGUAGE_ALIASES: Record<string, string> = {
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
  html: "markup",
  sql: "sql",
  yaml: "yaml",
  yml: "yaml",
  powershell: "powershell",
  pwsh: "powershell",
  ps1: "powershell",
  markdown: "markdown",
  md: "markdown",
};

function resolvePrismLanguage(language: string): string {
  const key = language.trim().toLowerCase();
  return PRISM_LANGUAGE_ALIASES[key] ?? "markup";
}

// Vite's dev server (esbuild) re-exports this CJS package's whole
// `module.exports` (i.e. {__esModule: true, default: Editor}) as its
// *own* default, rather than unwrapping it — so a plain default import
// gives us that wrapper object, not the component. Vite's production
// build (Rollup) unwraps it correctly and gives the component directly.
// Checking for a nested `.default` handles both: in dev it finds the
// real component one level in; in prod there's no nested `.default` on
// the component itself, so it falls through to the import as-is.
const Editor =
  (ReactSimpleCodeEditor as unknown as { default?: typeof ReactSimpleCodeEditor })
    .default ?? ReactSimpleCodeEditor;

// A scratch space for editing a copy of a snippet's code without saving
// anything back to it. Deliberately laid out like the detail page (same
// back link, same meta row with language/tags/copy) so it reads as "the
// same snippet, but editable" rather than a disconnected tool — the only
// difference is the code area is a live, syntax-highlighted editor, and
// the copy button copies whatever you've typed rather than the original.
export function TempEditPage() {
  const { id } = useParams<{ id: string }>();
  const location = useLocation();
  const backTo = (location.state as { from?: string } | null)?.from ?? "/";

  const [snippet, setSnippet] = useState<Snippet | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [code, setCode] = useState("");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!id) return;
    fetchSnippet(Number(id))
      .then((s) => {
        setSnippet(s);
        setCode(s.content);
      })
      .catch((err) => setError(err.message));
  }, [id]);

  async function handleCopy() {
    const ok = await copyToClipboard(code);
    if (ok) {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  if (error) return <p className="error">Error: {error}</p>;
  if (!snippet) return <p>Loading...</p>;

  const prismLanguage = resolvePrismLanguage(snippet.language);
  const grammar = Prism.languages[prismLanguage] ?? Prism.languages.markup;

  return (
    <div className="page">
      <Link to={backTo} className="back-link">
        &larr; Back to snippets
      </Link>

      <div className="page-header">
        <h1>Temporary Edit</h1>
      </div>

      <p className="temp-edit-hint">
        Edit this copy freely — nothing here is saved back to the
        snippet. Copy the result out when you're done.
      </p>

      <div className="detail-meta-row">
        <div className="tag-list">
          <span className="language-badge">{snippet.language}</span>
          {(snippet.tags ?? []).map((tag) => (
            <span key={tag} className="tag">
              {tag}
            </span>
          ))}
        </div>
        <button type="button" className="button" onClick={handleCopy}>
          {copied ? "Copied!" : "Copy"}
        </button>
      </div>

      <div className="temp-edit-editor-wrapper">
        <Editor
          value={code}
          onValueChange={setCode}
          highlight={(text) => Prism.highlight(text, grammar, prismLanguage)}
          padding={16}
          textareaId="temp-edit-textarea"
          className="temp-edit-editor"
          style={{
            fontFamily: '"SFMono-Regular", Consolas, monospace',
            fontSize: 14,
          }}
        />
      </div>
    </div>
  );
}
