import { useState } from "react";
import { Link, useLocation } from "react-router-dom";
import type { Snippet } from "../types";
import { CopyButton } from "./CopyButton";
import { CodeBlock } from "./CodeBlock";

interface SnippetCardProps {
  snippet: Snippet;
}

const COLLAPSED_LINE_COUNT = 3;

export function SnippetCard({ snippet }: SnippetCardProps) {
  const [expanded, setExpanded] = useState(false);
  const location = useLocation();

  // Code collapses to a fixed number of lines by default; the full body
  // is only rendered once the card is expanded. Whether it's worth
  // showing a toggle at all depends on whether it's actually longer than
  // the collapsed height.
  const lineCount = snippet.content.split("\n").length;
  const isTruncatable = lineCount > COLLAPSED_LINE_COUNT;

  // The current list URL (with its category/search state), passed along
  // as router state to both the detail and temp-edit pages, so their
  // back links return here instead of resetting to a blank list.
  const fromState = { from: location.pathname + location.search };

  return (
    // id lets the list page scroll back to this exact card after
    // returning from the detail page (see SnippetDetail's back link and
    // the scroll-to-hash effect in SnippetList).
    <div className="snippet-card" id={`snippet-${snippet.id}`}>
      <div className="snippet-card-body">
        <div className="snippet-card-row">
          {/* Selecting the description opens the full snippet page —
              that's where editing and deleting happen. */}
          <Link
            to={`/snippets/${snippet.id}`}
            state={fromState}
            className={
              "snippet-card-description" +
              (snippet.description ? "" : " snippet-card-description-empty")
            }
          >
            {snippet.description || "No description"}
          </Link>
          <div className="snippet-card-actions">
            <Link
              to={`/snippets/${snippet.id}/temp-edit`}
              state={fromState}
              className="copy-button copy-button-light"
              aria-label="Edit a temporary copy without saving"
              title="Edit a temporary copy without saving"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
                <path d="m15 5 4 4" />
              </svg>
            </Link>
            <CopyButton text={snippet.content} className="copy-button-light" />
          </div>
        </div>

        <div
          className={
            "code-wrapper code-wrapper-full-bleed" +
            (isTruncatable && !expanded ? " code-wrapper-collapsed" : "")
          }
        >
          <CodeBlock code={snippet.content} language={snippet.language} fullBleed />
          {isTruncatable && !expanded && <div className="code-fade" />}
        </div>

        {isTruncatable && (
          <button
            type="button"
            className="code-toggle"
            onClick={() => setExpanded((e) => !e)}
          >
            {expanded ? "Show less" : `Show all ${lineCount} lines`}
          </button>
        )}
      </div>
    </div>
  );
}
