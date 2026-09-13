import type { Snippet, SnippetInput } from "./types";

// All requests go to "/api/..." — during development, vite.config.ts
// proxies that to the Go server on :8080. In production, the Go server
// serves the built frontend itself, so "/api/..." is already same-origin.
const BASE_URL = "/api/snippets";

// Small helper so every function doesn't repeat the same
// "check response.ok, otherwise throw" logic.
async function handleResponse<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `Request failed with status ${res.status}`);
  }
  // DELETE returns 204 No Content — nothing to parse.
  if (res.status === 204) {
    return undefined as T;
  }
  return res.json() as Promise<T>;
}

export async function fetchSnippets(query?: string): Promise<Snippet[]> {
  const url = query ? `${BASE_URL}?q=${encodeURIComponent(query)}` : BASE_URL;
  const res = await fetch(url);
  return handleResponse<Snippet[]>(res);
}

export async function fetchSnippet(id: number): Promise<Snippet> {
  const res = await fetch(`${BASE_URL}/${id}`);
  return handleResponse<Snippet>(res);
}

export async function createSnippet(input: SnippetInput): Promise<Snippet> {
  const res = await fetch(BASE_URL, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return handleResponse<Snippet>(res);
}

export async function updateSnippet(
  id: number,
  input: SnippetInput
): Promise<Snippet> {
  const res = await fetch(`${BASE_URL}/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return handleResponse<Snippet>(res);
}

export async function deleteSnippet(id: number): Promise<void> {
  const res = await fetch(`${BASE_URL}/${id}`, { method: "DELETE" });
  return handleResponse<void>(res);
}

export async function updateSnippetOrder(
  id: number,
  order: number
): Promise<Snippet> {
  const res = await fetch(`${BASE_URL}/${id}/order`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ order }),
  });
  return handleResponse<Snippet>(res);
}

// --- Auth ---
// These hit /api/login, /api/logout, etc. rather than /api/snippets/...,
// so they don't use BASE_URL above.

export async function checkAuth(): Promise<boolean> {
  const res = await fetch("/api/auth/check");
  return res.ok;
}

export async function login(password: string): Promise<void> {
  const res = await fetch("/api/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ password }),
  });
  return handleResponse<void>(res);
}

export async function logout(): Promise<void> {
  await fetch("/api/logout", { method: "POST" });
}

export async function changePassword(
  currentPassword: string,
  newPassword: string
): Promise<void> {
  const res = await fetch("/api/settings/password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ currentPassword, newPassword }),
  });
  return handleResponse<void>(res);
}
