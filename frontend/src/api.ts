import { requestReauth } from "./session";
import type { Snippet, SnippetInput } from "./types";

// All requests go to "/api/..." — during development, vite.config.ts
// proxies that to the Go server on :8080. In production, the Go server
// serves the built frontend itself, so "/api/..." is already same-origin.
const BASE_URL = "/api/snippets";

// Asks the server whether our session cookie is still good. Only a
// definite 401 counts as "expired": a network blip or a 5xx here proves
// nothing about the session, so those return false.
async function sessionHasExpired(): Promise<boolean> {
  try {
    const res = await fetch("/api/auth/check");
    return res.status === 401;
  } catch {
    return false;
  }
}

// fetch() for every call that needs a logged-in session.
//
// Sessions only live in the server's memory, so a server restart (or a
// password change from another device) silently ends them. Instead of
// letting the next click fail with "not logged in", a 401 here:
//   1. asks the server whether the session really is gone — a 401 can
//      also be an ordinary refusal, e.g. "current password is incorrect"
//      from the change-password form, which must reach the caller as-is;
//   2. if it is gone, asks the UI to show the login overlay and waits;
//   3. once the user has logged in, sends the original request again.
// Retrying is safe: the server rejects an unauthenticated request before
// doing any work, so the first attempt never half-happened. Request
// bodies are plain strings, so they can be sent a second time.
async function authedFetch(input: string, init?: RequestInit): Promise<Response> {
  const res = await fetch(input, init);
  if (res.status !== 401) return res;
  if (!(await sessionHasExpired())) return res;
  await requestReauth();
  return fetch(input, init);
}

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
  const res = await authedFetch(url);
  return handleResponse<Snippet[]>(res);
}

export async function fetchSnippet(id: number): Promise<Snippet> {
  const res = await authedFetch(`${BASE_URL}/${id}`);
  return handleResponse<Snippet>(res);
}

export async function createSnippet(input: SnippetInput): Promise<Snippet> {
  const res = await authedFetch(BASE_URL, {
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
  const res = await authedFetch(`${BASE_URL}/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  return handleResponse<Snippet>(res);
}

export async function deleteSnippet(id: number): Promise<void> {
  const res = await authedFetch(`${BASE_URL}/${id}`, { method: "DELETE" });
  return handleResponse<void>(res);
}

export async function updateSnippetOrder(
  id: number,
  order: number
): Promise<Snippet> {
  const res = await authedFetch(`${BASE_URL}/${id}/order`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ order }),
  });
  return handleResponse<Snippet>(res);
}

// --- Auth ---
// These hit /api/login, /api/logout, etc. rather than /api/snippets/...,
// so they don't use BASE_URL above. login/logout/checkAuth deliberately
// use plain fetch: a 401 from them is an answer, not an expired session.

// Resolves true (logged in) or false (not logged in). Throws if the server
// can't give a definite answer — it's down, or returned a 5xx — so the
// caller can show "can't reach the server" instead of wrongly showing the
// login form (or, as before, hanging on a blank screen).
export async function checkAuth(): Promise<boolean> {
  const res = await fetch("/api/auth/check");
  if (res.ok) return true;
  if (res.status === 401) return false;
  throw new Error(`Server returned ${res.status}`);
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
  const res = await authedFetch("/api/settings/password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ currentPassword, newPassword }),
  });
  return handleResponse<void>(res);
}
