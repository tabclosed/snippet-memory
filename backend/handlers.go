package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// api bundles the dependencies HTTP handlers need. Using a struct with
// methods (instead of loose functions with global variables) keeps the
// store easy to pass around and easy to test later.
type api struct {
	store *Store
}

// maxSnippetBodyBytes caps how large a single create/update request body
// can be. Generous for actual code snippets, but stops someone (or a
// buggy script) from sending an enormous body and exhausting memory.
const maxSnippetBodyBytes = 5 * 1024 * 1024 // 5 MB

// writeJSON is a small helper to avoid repeating the same three lines
// (set header, set status, encode) in every handler.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// listSnippets handles GET /api/snippets — returns every snippet.
// Supports an optional ?q= query param for simple text search across
// description, language, and tags.
func (a *api) listSnippets(w http.ResponseWriter, r *http.Request) {
	all := a.store.All()

	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusOK, all)
		return
	}

	filtered := filterSnippets(all, query)
	writeJSON(w, http.StatusOK, filtered)
}

// getSnippet handles GET /api/snippets/{id} — returns one snippet.
func (a *api) getSnippet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	snip, ok := a.store.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}
	writeJSON(w, http.StatusOK, snip)
}

// createSnippet handles POST /api/snippets — creates a new snippet from
// a JSON request body.
func (a *api) createSnippet(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSnippetBodyBytes)
	var input SnippetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	snip, err := a.store.Create(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save snippet")
		return
	}
	writeJSON(w, http.StatusCreated, snip)
}

// updateSnippet handles PUT /api/snippets/{id} — replaces an existing
// snippet's fields.
func (a *api) updateSnippet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSnippetBodyBytes)
	var input SnippetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	snip, found, err := a.store.Update(id, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update snippet")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}
	writeJSON(w, http.StatusOK, snip)
}

// updateSnippetOrder handles PATCH /api/snippets/{id}/order — sets just
// the display-order value for one snippet. The frontend calls this twice
// (once per snippet) to swap two adjacent cards.
func (a *api) updateSnippetOrder(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var body struct {
		Order int `json:"order"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	snip, found, err := a.store.SetOrder(id, body.Order)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not update order")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}
	writeJSON(w, http.StatusOK, snip)
}

// deleteSnippet handles DELETE /api/snippets/{id}.
func (a *api) deleteSnippet(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	found, err := a.store.Delete(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete snippet")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "snippet not found")
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}
