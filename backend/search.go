package main

import "strings"

// filterSnippets returns the snippets whose description, language, or
// tags contain the query text (case-insensitive substring match).
// Simple on purpose — a real search index isn't needed at this scale.
func filterSnippets(snippets []Snippet, query string) []Snippet {
	q := strings.ToLower(query)
	result := make([]Snippet, 0)

	for _, snip := range snippets {
		if strings.Contains(strings.ToLower(snip.Description), q) ||
			strings.Contains(strings.ToLower(snip.Language), q) ||
			containsTag(snip.Tags, q) {
			result = append(result, snip)
		}
	}
	return result
}

func containsTag(tags []string, q string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}
