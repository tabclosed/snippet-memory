import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import { fetchSnippets } from "../api";
import type { Snippet } from "../types";
import { SnippetCard } from "../components/SnippetCard";

// Sentinel value for the synthetic "Untagged" category (snippets with an
// empty tags array). Not a real tag, so it can't collide with one.
const UNTAGGED = "__untagged__";

export function SnippetList() {
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Category and search state live in the URL (?category=...&q=...)
  // rather than plain useState — that way the current view is part of
  // the browser history entry. When you open a snippet and then go back,
  // the back link returns to this exact URL, landing you back on the
  // same category/search instead of a reset, empty list.
  const [searchParams, setSearchParams] = useSearchParams();
  const selectedCategory = searchParams.get("category");
  const query = searchParams.get("q") ?? "";

  function setSelectedCategory(category: string | null) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (category) next.set("category", category);
        else next.delete("category");
        return next;
      },
      { replace: true }
    );
  }

  function setQuery(q: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        if (q) next.set("q", q);
        else next.delete("q");
        return next;
      },
      { replace: true }
    );
  }

  // Fetch the full list once. Filtering by search text and by category
  // both happen client-side below, so we don't need to re-fetch on every
  // keystroke or every category click.
  useEffect(() => {
    fetchSnippets()
      .then((data) => {
        setSnippets(data);
        setError(null);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  // Categories = every unique tag across all snippets, sorted alphabetically.
  // useMemo avoids recomputing this on every render — only when the
  // underlying snippet list actually changes.
  const categories = useMemo(() => {
    const tagSet = new Set<string>();
    for (const snippet of snippets) {
      for (const tag of snippet.tags) {
        tagSet.add(tag);
      }
    }
    return Array.from(tagSet).sort((a, b) => a.localeCompare(b));
  }, [snippets]);

  // Snippets with no tags at all fall into the synthetic "Untagged"
  // category rather than being hidden or lumped under a catch-all "All".
  const untaggedCount = useMemo(
    () => snippets.filter((s) => s.tags.length === 0).length,
    [snippets]
  );

  const filteredSnippets = useMemo(() => {
    if (!selectedCategory && !query) return [];

    return snippets
      .filter((snippet) => {
        const matchesCategory =
          !selectedCategory ||
          (selectedCategory === UNTAGGED
            ? snippet.tags.length === 0
            : snippet.tags.includes(selectedCategory));

        if (!matchesCategory) return false;

        if (!query) return true;
        const q = query.toLowerCase();
        return (
          snippet.description.toLowerCase().includes(q) ||
          snippet.language.toLowerCase().includes(q) ||
          snippet.tags.some((tag) => tag.toLowerCase().includes(q))
        );
      })
      .sort((a, b) => a.order - b.order);
  }, [snippets, query, selectedCategory]);

  // When arriving here via a "#snippet-<id>" hash (SnippetDetail's back
  // link sets one), scroll straight to that card instead of just landing
  // at the top of the category. Keyed off location.key so it only fires
  // once per navigation — and retried across renders via the
  // filteredSnippets dependency, since the card doesn't exist in the DOM
  // until the fetched data has rendered.
  const location = useLocation();
  const scrolledForKey = useRef<string | null>(null);

  useEffect(() => {
    if (!location.hash || scrolledForKey.current === location.key) return;
    const target = document.getElementById(location.hash.slice(1));
    if (target) {
      target.scrollIntoView({ behavior: "smooth", block: "center" });
      scrolledForKey.current = location.key;
    }
  }, [location, filteredSnippets]);

  return (
    <div className="page">
      <div className="page-header">
        <div className="page-header-left">
          <h1>Snippets</h1>
          <Link
            to="/settings"
            className="copy-button copy-button-light settings-button"
            aria-label="Settings"
            title="Settings"
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
              <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
          </Link>
        </div>
        <Link to="/snippets/new" className="button">
          + New Snippet
        </Link>
      </div>

      <div className="list-layout">
        <aside className="sidebar">
          <h2>Categories</h2>
          <ul className="category-list">
            {categories.map((category) => {
              const count = snippets.filter((s) =>
                s.tags.includes(category)
              ).length;
              return (
                <li key={category}>
                  <button
                    className={
                      "category-item" +
                      (selectedCategory === category ? " active" : "")
                    }
                    onClick={() =>
                      setSelectedCategory(
                        selectedCategory === category ? null : category
                      )
                    }
                  >
                    {category}
                    <span className="category-count">{count}</span>
                  </button>
                </li>
              );
            })}
            {untaggedCount > 0 && (
              <li>
                <button
                  className={
                    "category-item" +
                    (selectedCategory === UNTAGGED ? " active" : "")
                  }
                  onClick={() =>
                    setSelectedCategory(
                      selectedCategory === UNTAGGED ? null : UNTAGGED
                    )
                  }
                >
                  Untagged
                  <span className="category-count">{untaggedCount}</span>
                </button>
              </li>
            )}
          </ul>
        </aside>

        <div className="main-content">
          <input
            type="text"
            placeholder="Search by description, tag, or language..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="search-input"
          />

          {loading && <p>Loading...</p>}
          {error && <p className="error">Error: {error}</p>}

          {!loading && !error && !selectedCategory && !query && (
            <p className="empty-state">
              Select a tag or search to view snippets.
            </p>
          )}

          {!loading &&
            !error &&
            (selectedCategory || query) &&
            filteredSnippets.length === 0 && (
              <p className="empty-state">No snippets found.</p>
            )}

          <div className="snippet-list">
            {filteredSnippets.map((snippet) => (
              <SnippetCard key={snippet.id} snippet={snippet} />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
