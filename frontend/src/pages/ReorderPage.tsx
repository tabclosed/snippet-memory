import { useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { fetchSnippets, updateSnippetOrder } from "../api";
import type { Snippet } from "../types";

const UNTAGGED = "__untagged__";

// Settings > Reorder Snippets: pick a category, then drag snippets into
// place (or use the arrow/top/bottom buttons). This is the only place
// reordering happens now — the main screen just displays whatever order
// was set here.
export function ReorderPage() {
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedCategory, setSelectedCategory] = useState<string | null>(
    null
  );

  // Drag state lives outside React's render data (it's purely a UI
  // concern, not something that needs to trigger the memoized filtering
  // below) — dragIndex is the row being picked up, dragOverIndex is
  // whichever row the pointer is currently over, used just to draw a
  // highlight showing where it would land.
  const dragIndex = useRef<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  useEffect(() => {
    fetchSnippets()
      .then(setSnippets)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const categories = useMemo(() => {
    const tagSet = new Set<string>();
    for (const snippet of snippets) {
      for (const tag of snippet.tags) tagSet.add(tag);
    }
    return Array.from(tagSet).sort((a, b) => a.localeCompare(b));
  }, [snippets]);

  const untaggedCount = useMemo(
    () => snippets.filter((s) => s.tags.length === 0).length,
    [snippets]
  );

  const categorySnippets = useMemo(() => {
    if (!selectedCategory) return [];
    return snippets
      .filter((s) =>
        selectedCategory === UNTAGGED
          ? s.tags.length === 0
          : s.tags.includes(selectedCategory)
      )
      .sort((a, b) => a.order - b.order);
  }, [snippets, selectedCategory]);

  // The general-purpose move: takes whichever Order values this
  // category's snippets currently hold, and reassigns them to whatever
  // new arrangement the snippets end up in — so up/down, jump-to-top/
  // bottom, and dragging to an arbitrary spot are all just different
  // (fromIndex, toIndex) pairs handled by the same code, and no other
  // category's Order values are touched.
  async function moveSnippet(fromIndex: number, toIndex: number) {
    if (fromIndex === toIndex) return;
    if (toIndex < 0 || toIndex >= categorySnippets.length) return;

    const orderSlots = categorySnippets.map((s) => s.order); // ascending
    const rearranged = [...categorySnippets];
    const [moved] = rearranged.splice(fromIndex, 1);
    rearranged.splice(toIndex, 0, moved);

    const updates = rearranged.map((s, i) => ({ id: s.id, order: orderSlots[i] }));
    const nextOrderById = new Map(updates.map((u) => [u.id, u.order]));

    setSnippets((prev) =>
      prev.map((s) =>
        nextOrderById.has(s.id) ? { ...s, order: nextOrderById.get(s.id)! } : s
      )
    );

    try {
      await Promise.all(updates.map((u) => updateSnippetOrder(u.id, u.order)));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reorder");
      fetchSnippets()
        .then(setSnippets)
        .catch(() => {});
    }
  }

  function handleDrop(dropIndex: number) {
    setDragOverIndex(null);
    const from = dragIndex.current;
    dragIndex.current = null;
    if (from === null) return;
    moveSnippet(from, dropIndex);
  }

  return (
    <div className="page">
      <Link to="/settings" className="back-link">
        &larr; Back to settings
      </Link>

      <div className="page-header">
        <h1>Reorder Snippets</h1>
      </div>

      <p className="reorder-hint">
        Pick a category, then drag a snippet into place, or use the
        buttons to nudge it up/down or jump it to the top/bottom.
      </p>

      {error && <p className="error">Error: {error}</p>}

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
                    onClick={() => setSelectedCategory(category)}
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
                  onClick={() => setSelectedCategory(UNTAGGED)}
                >
                  Untagged
                  <span className="category-count">{untaggedCount}</span>
                </button>
              </li>
            )}
          </ul>
        </aside>

        <div className="main-content">
          {loading && <p>Loading...</p>}

          {!loading && !selectedCategory && (
            <p className="empty-state">
              Select a category to reorder its snippets.
            </p>
          )}

          {!loading && selectedCategory && categorySnippets.length === 0 && (
            <p className="empty-state">No snippets in this category.</p>
          )}

          <ul className="reorder-list">
            {categorySnippets.map((snippet, index) => (
              <li
                key={snippet.id}
                className={
                  "reorder-row" +
                  (dragOverIndex === index ? " reorder-row-drag-over" : "")
                }
                draggable
                onDragStart={() => {
                  dragIndex.current = index;
                }}
                onDragOver={(e) => {
                  e.preventDefault(); // required to allow a drop at all
                  if (dragOverIndex !== index) setDragOverIndex(index);
                }}
                onDragLeave={() => {
                  setDragOverIndex((current) => (current === index ? null : current));
                }}
                onDrop={(e) => {
                  e.preventDefault();
                  handleDrop(index);
                }}
                onDragEnd={() => {
                  dragIndex.current = null;
                  setDragOverIndex(null);
                }}
              >
                <span className="reorder-row-handle" aria-hidden="true">
                  ⠿
                </span>

                <span
                  className={
                    "reorder-row-description" +
                    (snippet.description
                      ? ""
                      : " reorder-row-description-empty")
                  }
                >
                  {snippet.description || "No description"}
                </span>

                <div className="reorder-row-buttons">
                  <button
                    type="button"
                    className="reorder-button"
                    onClick={() => moveSnippet(index, 0)}
                    disabled={index === 0}
                    aria-label="Move snippet to top"
                    title="Move to top"
                  >
                    ⤒
                  </button>
                  <button
                    type="button"
                    className="reorder-button"
                    onClick={() => moveSnippet(index, index - 1)}
                    disabled={index === 0}
                    aria-label="Move snippet up"
                    title="Move up"
                  >
                    ▲
                  </button>
                  <button
                    type="button"
                    className="reorder-button"
                    onClick={() => moveSnippet(index, index + 1)}
                    disabled={index === categorySnippets.length - 1}
                    aria-label="Move snippet down"
                    title="Move down"
                  >
                    ▼
                  </button>
                  <button
                    type="button"
                    className="reorder-button"
                    onClick={() => moveSnippet(index, categorySnippets.length - 1)}
                    disabled={index === categorySnippets.length - 1}
                    aria-label="Move snippet to bottom"
                    title="Move to bottom"
                  >
                    ⤓
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </div>
  );
}
