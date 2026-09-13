import { useEffect, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { createSnippet, fetchSnippet, updateSnippet } from "../api";
import type { SnippetInput } from "../types";

const emptyForm: SnippetInput = {
  description: "",
  language: "",
  tags: [],
  content: "",
};

export function SnippetForm() {
  // If there's an :id in the URL, we're editing an existing snippet;
  // otherwise this is the "new snippet" form. Same component, two modes.
  const { id } = useParams<{ id: string }>();
  const isEditing = Boolean(id);
  const navigate = useNavigate();
  const location = useLocation();

  // Passed along from the detail page's Edit link (see SnippetDetail.tsx)
  // — the list URL to eventually return to, so cancelling or saving
  // doesn't lose track of which category/search was open.
  const from = (location.state as { from?: string } | null)?.from;

  const [form, setForm] = useState<SnippetInput>(emptyForm);
  // Tags are edited as a single comma-separated string in the input box,
  // then split into an array only when submitting — much simpler than
  // building a tag-chip UI for a first version.
  const [tagsText, setTagsText] = useState("");
  const [loading, setLoading] = useState(isEditing);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isEditing || !id) return;
    fetchSnippet(Number(id))
      .then((snippet) => {
        setForm({
          description: snippet.description,
          language: snippet.language,
          tags: snippet.tags,
          content: snippet.content,
        });
        setTagsText(snippet.tags.join(", "));
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id, isEditing]);

  // A single change handler for every text/textarea field, keyed by
  // the input's `name` attribute — avoids writing one handler per field.
  function handleChange(
    e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>
  ) {
    const { name, value } = e.target;
    setForm((prev) => ({ ...prev, [name]: value }));
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);

    const tags = tagsText
      .split(",")
      .map((t) => t.trim())
      .filter(Boolean);
    const payload: SnippetInput = { ...form, tags };

    try {
      const saved =
        isEditing && id
          ? await updateSnippet(Number(id), payload)
          : await createSnippet(payload);
      navigate(`/snippets/${saved.id}`, { state: { from } });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save");
      setSaving(false);
    }
  }

  function handleCancel() {
    // Editing an existing snippet -> go back to its detail page (with
    // the same list state it was opened from). Creating a new one ->
    // nothing to go back to, so go home.
    if (isEditing && id) {
      navigate(`/snippets/${id}`, { state: { from } });
    } else {
      navigate("/");
    }
  }

  if (loading) return <p>Loading...</p>;

  return (
    <div className="page">
      <h1>{isEditing ? "Edit Snippet" : "New Snippet"}</h1>

      {error && <p className="error">Error: {error}</p>}

      <form onSubmit={handleSubmit} className="snippet-form">
        <label>
          Description
          <input
            name="description"
            value={form.description}
            onChange={handleChange}
          />
        </label>

        <label>
          Language
          <input
            name="language"
            value={form.language}
            onChange={handleChange}
            placeholder="e.g. go, typescript, python"
          />
        </label>

        <label>
          Tags (comma-separated)
          <input
            name="tags"
            value={tagsText}
            onChange={(e) => setTagsText(e.target.value)}
            placeholder="e.g. basics, example"
          />
        </label>

        <label>
          Content
          <textarea
            name="content"
            value={form.content}
            onChange={handleChange}
            rows={12}
            required
          />
        </label>

        <div className="button-group">
          <button type="submit" className="button" disabled={saving}>
            {saving ? "Saving..." : "Save"}
          </button>
          <button
            type="button"
            className="button button-secondary"
            onClick={handleCancel}
            disabled={saving}
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  );
}
