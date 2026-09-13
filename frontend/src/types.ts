// This mirrors the Go `Snippet` struct in backend/models.go. Keeping the
// shape identical on both sides is what makes the JSON travelling between
// them predictable — TypeScript just describes what the Go server already
// sends.
export interface Snippet {
  id: number;
  description: string;
  language: string;
  tags: string[];
  content: string;
  order: number;
  createdAt: string; // ISO date string — JSON has no native date type
  updatedAt: string;
}

// Mirrors Go's `SnippetInput` — the fields the *user* provides when
// creating or editing a snippet. Notably missing: id, createdAt, updatedAt.
// The server fills those in; the client never sends them.
export interface SnippetInput {
  description: string;
  language: string;
  tags: string[];
  content: string;
}
