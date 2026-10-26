import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { changePassword, checkAuth, createSnippet, fetchSnippets, login } from "../api";
import { completeReauth, isReauthPending } from "../session";

type Reply = Response | Error;
const json = (status: number, body: unknown = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

// Queue-per-URL fetch mock: each call to a URL consumes its next reply.
function mockFetch(routes: Record<string, Reply[]>) {
  const calls: string[] = [];
  const fn = vi.fn(async (input: RequestInfo | URL, _init?: RequestInit) => {
    const url = String(input);
    calls.push(url);
    const queue = routes[url];
    if (!queue || queue.length === 0) throw new Error(`unexpected fetch ${url}`);
    const next = queue.length > 1 ? queue.shift()! : queue[0];
    if (next instanceof Error) throw next;
    return next.clone();
  });
  vi.stubGlobal("fetch", fn);
  return { fn, calls };
}

const tick = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  if (isReauthPending()) completeReauth();
});
afterEach(() => {
  vi.unstubAllGlobals();
  if (isReauthPending()) completeReauth();
});

describe("expired session handling", () => {
  it("passes ordinary responses straight through", async () => {
    const { calls } = mockFetch({ "/api/snippets": [json(200, [])] });
    await expect(fetchSnippets()).resolves.toEqual([]);
    expect(calls).toEqual(["/api/snippets"]);
    expect(isReauthPending()).toBe(false);
  });

  it("on a real expiry: asks for re-login, waits, then retries the request", async () => {
    const { calls } = mockFetch({
      "/api/snippets": [json(401, { error: "not logged in" }), json(200, [{ id: 1 }])],
      "/api/auth/check": [json(401)],
    });
    let settled = false;
    const result = fetchSnippets().then((r) => {
      settled = true;
      return r;
    });

    await tick();
    expect(isReauthPending()).toBe(true);
    expect(settled).toBe(false); // still waiting on the user
    expect(calls).toEqual(["/api/snippets", "/api/auth/check"]);

    completeReauth();
    await expect(result).resolves.toEqual([{ id: 1 }]);
    expect(calls).toEqual(["/api/snippets", "/api/auth/check", "/api/snippets"]);
    expect(isReauthPending()).toBe(false);
  });

  it("retries a write with the same method and body", async () => {
    const { fn } = mockFetch({
      "/api/snippets": [json(401), json(201, { id: 7 })],
      "/api/auth/check": [json(401)],
    });
    const input = { description: "d", language: "go", tags: ["a"], content: "x" };
    const p = createSnippet(input);
    await tick();
    completeReauth();
    await expect(p).resolves.toEqual({ id: 7 });
    const posts = fn.mock.calls.filter(([u]) => u === "/api/snippets");
    expect(posts).toHaveLength(2);
    for (const [, init] of posts) {
      expect(init?.method).toBe("POST");
      expect(init?.body).toBe(JSON.stringify(input));
    }
  });

  it("concurrent requests share ONE re-login and all retry afterwards", async () => {
    const { calls } = mockFetch({
      "/api/snippets": [json(401), json(401), json(200, [])],
      "/api/snippets/1": [json(401), json(200, { id: 1 })],
      "/api/auth/check": [json(401)],
    });
    const a = fetchSnippets();
    const b = fetchSnippets();
    await tick();
    expect(isReauthPending()).toBe(true);
    completeReauth(); // a single login releases everyone
    await Promise.all([a, b]);
    expect(calls.filter((u) => u === "/api/snippets").length).toBe(4);
  });

  it("a 401 that is NOT an expired session reaches the caller unchanged", async () => {
    // e.g. wrong "current password" in Settings: the session is fine.
    mockFetch({
      "/api/settings/password": [json(401, { error: "current password is incorrect" })],
      "/api/auth/check": [json(200, { ok: true })],
    });
    await expect(changePassword("bad", "newpass")).rejects.toThrow("current password is incorrect");
    expect(isReauthPending()).toBe(false);
  });

  it("does not show the login overlay when the server can't be reached", async () => {
    mockFetch({
      "/api/snippets": [json(401)],
      "/api/auth/check": [new TypeError("network down")],
    });
    await expect(fetchSnippets()).rejects.toThrow();
    expect(isReauthPending()).toBe(false);
  });

  it("does not treat a 5xx on the session check as expiry", async () => {
    mockFetch({ "/api/snippets": [json(401)], "/api/auth/check": [json(503)] });
    await expect(fetchSnippets()).rejects.toThrow();
    expect(isReauthPending()).toBe(false);
  });

  it("login failures are plain errors, never a re-login prompt", async () => {
    mockFetch({ "/api/login": [json(401, { error: "incorrect password" })] });
    await expect(login("x")).rejects.toThrow("incorrect password");
    expect(isReauthPending()).toBe(false);
  });

  it("only retries once: a second 401 after logging in is returned, not looped", async () => {
    const { calls } = mockFetch({
      "/api/snippets": [json(401)],
      "/api/auth/check": [json(401)],
    });
    const p = fetchSnippets();
    await tick();
    completeReauth();
    await expect(p).rejects.toThrow();
    expect(calls.filter((u) => u === "/api/snippets").length).toBe(2);
  });
});

describe("checkAuth", () => {
  it("true when logged in, false on 401", async () => {
    mockFetch({ "/api/auth/check": [json(200)] });
    await expect(checkAuth()).resolves.toBe(true);
    mockFetch({ "/api/auth/check": [json(401)] });
    await expect(checkAuth()).resolves.toBe(false);
  });

  it("throws (instead of hanging or lying) when the server gives no answer", async () => {
    mockFetch({ "/api/auth/check": [json(502)] });
    await expect(checkAuth()).rejects.toThrow("502");
    mockFetch({ "/api/auth/check": [new TypeError("Failed to fetch")] });
    await expect(checkAuth()).rejects.toThrow();
  });
});
