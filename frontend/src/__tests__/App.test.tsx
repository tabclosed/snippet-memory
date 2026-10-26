import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "../App";
import { completeReauth, isReauthPending } from "../session";

const json = (status: number, body: unknown = {}) =>
  new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });

const snippet = {
  id: 5, description: "saved", language: "go", tags: [], content: "x",
  order: 0, createdAt: "2026-01-01T00:00:00Z", updatedAt: "2026-01-01T00:00:00Z",
};

beforeEach(() => {
  window.history.pushState({}, "", "/");
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.useRealTimers();
  if (isReauthPending()) completeReauth();
});

describe("startup", () => {
  it("shows the login form when not logged in", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => json(401)));
    render(<App />);
    expect(await screen.findByPlaceholderText("Password")).toBeTruthy();
    expect(screen.queryByText(/Your session expired/)).toBeNull();
  });

  it("no longer hangs on a blank screen when the server is down: shows a message and recovers", async () => {
    let up = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/api/auth/check") {
        if (!up) throw new TypeError("Failed to fetch");
        return json(401);
      }
      return json(200, []);
    });
    vi.stubGlobal("fetch", fetchMock);
    render(<App />);

    expect(await screen.findByText("Can't reach the server.")).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toMatch(/Trying again/);

    up = true; // server comes back
    await userEvent.click(screen.getByRole("button", { name: "Try again now" }));
    expect(await screen.findByPlaceholderText("Password")).toBeTruthy();
  });

  it("retries on its own while the server is unreachable", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let up = false;
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => {
      if (String(input) === "/api/auth/check") {
        if (!up) return json(503);
        return json(200);
      }
      return json(200, []);
    }));
    render(<App />);
    expect(await screen.findByText("Can't reach the server.")).toBeTruthy();
    up = true;
    await act(async () => { await vi.advanceTimersByTimeAsync(3500); });
    await waitFor(() => expect(screen.queryByText("Can't reach the server.")).toBeNull());
  });
});

describe("session expires while using the app", () => {
  it("keeps the unsaved form, asks for login, then completes the save", async () => {
    window.history.pushState({}, "", "/snippets/new");

    let sessionValid = true;
    const posts: string[] = [];
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method ?? "GET";
      if (url === "/api/auth/check") return json(sessionValid ? 200 : 401);
      if (url === "/api/login") { sessionValid = true; return json(200, { ok: true }); }
      if (url === "/api/snippets" && method === "POST") {
        if (!sessionValid) return json(401, { error: "not logged in" });
        posts.push(String(init?.body));
        return json(201, snippet);
      }
      if (url === "/api/snippets/5") return json(200, snippet);
      return json(200, []);
    }));

    const user = userEvent.setup();
    render(<App />);

    const content = (await screen.findByLabelText(/Content/)) as HTMLTextAreaElement;
    await user.type(content, "important work");
    await user.type(screen.getByLabelText(/Description/), "my snippet");

    // The server restarts: the session is gone.
    sessionValid = false;
    await user.click(screen.getByRole("button", { name: "Save" }));

    // Login overlay appears over the app...
    const dialog = await screen.findByRole("dialog", { name: "Session expired" });
    expect(dialog.textContent).toMatch(/Your session expired/);
    // ...and the work in progress is still there underneath, not wiped.
    expect(content.value).toBe("important work");
    expect(content.closest("[inert]")).not.toBeNull(); // but can't be interacted with
    expect(posts).toHaveLength(0);

    await user.type(within(dialog).getByPlaceholderText("Password"), "pw");
    await user.click(within(dialog).getByRole("button", { name: "Log in" }));

    // Overlay closes and the save that was interrupted goes through, once.
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    await waitFor(() => expect(posts).toHaveLength(1));
    expect(JSON.parse(posts[0]).content).toBe("important work");
    expect(JSON.parse(posts[0]).description).toBe("my snippet");
  });

  it("shows a wrong-password error in the overlay and stays until login succeeds", async () => {
    window.history.pushState({}, "", "/snippets/new");
    let sessionValid = true;
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url === "/api/auth/check") return json(sessionValid ? 200 : 401);
      if (url === "/api/login") {
        const pw = JSON.parse(String(init?.body)).password;
        if (pw !== "right") return json(401, { error: "incorrect password" });
        sessionValid = true;
        return json(200, { ok: true });
      }
      if (url === "/api/snippets" && init?.method === "POST") {
        return sessionValid ? json(201, snippet) : json(401);
      }
      return json(200, snippet);
    }));

    const user = userEvent.setup();
    render(<App />);
    await user.type(await screen.findByLabelText(/Content/), "abc");
    sessionValid = false;
    await user.click(screen.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByPlaceholderText("Password"), "wrong");
    await user.click(within(dialog).getByRole("button", { name: "Log in" }));
    expect(await within(dialog).findByText("incorrect password")).toBeTruthy();
    expect(screen.getByRole("dialog")).toBeTruthy();

    await user.clear(within(dialog).getByPlaceholderText("Password"));
    await user.type(within(dialog).getByPlaceholderText("Password"), "right");
    await user.click(within(dialog).getByRole("button", { name: "Log in" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
  });
});
