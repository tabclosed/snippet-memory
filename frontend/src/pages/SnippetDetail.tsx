import { useEffect, useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { deleteSnippet, fetchSnippet } from "../api";
import type { Snippet } from "../types";
import { CopyButton } from "../components/CopyButton";
import { CodeBlock } from "../components/CodeBlock";

export function SnippetDetail() {
  // useParams reads the dynamic part of the URL — for a route defined
  // as "/snippets/:id", visiting "/snippets/3" gives us params.id === "3".
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const location = useLocation();

  // SnippetCard passes along the list URL it was clicked from (including
  // its ?category=/?q= state) as router state. Falling back to "/" only
  // matters if this page was opened directly (e.g. a bookmark or refresh).
  const backTo = (location.state as { from?: string } | null)?.from ?? "/";

  // Appending #snippet-<id> to the back link lets the list page scroll
  // straight to this card instead of just landing at the top of
  // whichever category/search was open (see the scroll-to-hash effect
  // in SnippetList.tsx).
  const backToWithScroll = id ? `${backTo}#snippet-${id}` : backTo;

  const [snippet, setSnippet] = useState<Snippet | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    fetchSnippet(Number(id))
      .then(setSnippet)
      .catch((err) => setError(err.message));
  }, [id]);

  async function handleDelete() {
    if (!snippet) return;
    if (!confirm("Delete this snippet? This can't be undone.")) return;

    try {
      await deleteSnippet(snippet.id);
      navigate(backTo);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete");
    }
  }

  if (error) return <p className="error">Error: {error}</p>;
  if (!snippet) return <p>Loading...</p>;

  return (
    <div className="page">
      <Link to={backToWithScroll} className="back-link">
        &larr; Back to snippets
      </Link>

      <div className="page-header">
        <h1>{snippet.description || "Untitled snippet"}</h1>
        <div className="button-group">
          <Link
            to={`/snippets/${snippet.id}/edit`}
            state={{ from: backTo }}
            className="button"
          >
            Edit
          </Link>
          <button onClick={handleDelete} className="button button-danger">
            Delete
          </button>
        </div>
      </div>

      <div className="detail-meta-row">
        <div className="tag-list">
          <span className="language-badge">{snippet.language}</span>
          {(snippet.tags ?? []).map((tag) => (
            <span key={tag} className="tag">
              {tag}
            </span>
          ))}
        </div>
        <CopyButton text={snippet.content} className="copy-button-light" />
      </div>

      <div className="code-wrapper">
        <CodeBlock code={snippet.content} language={snippet.language} scrollable />
      </div>
    </div>
  );
}
