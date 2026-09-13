package main

import "time"

// Snippet is the core data structure of the app — one saved piece of code
// plus its metadata. The `json:"..."` tags control the field names used
// when this struct is encoded/decoded to/from JSON (both in the data file
// and in HTTP request/response bodies).
type Snippet struct {
	ID          int       `json:"id"`
	Description string    `json:"description"`
	Language    string    `json:"language"`
	Tags        []string  `json:"tags"`
	Content     string    `json:"content"`
	// Order controls display position on the main screen — lower values
	// sort first. Managed separately from the rest of the snippet's
	// fields via the up/down reorder buttons, not the create/edit form.
	Order     int       `json:"order"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// SnippetInput is what we expect the frontend to send when creating or
// updating a snippet. It's deliberately a separate type from Snippet:
// the client shouldn't be able to set id/createdAt/updatedAt itself —
// the server controls those.
type SnippetInput struct {
	Description string   `json:"description"`
	Language    string   `json:"language"`
	Tags        []string `json:"tags"`
	Content     string   `json:"content"`
}
