import { useCallback, useEffect, useState, useSyncExternalStore } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { checkAuth } from "./api";
import { completeReauth, isReauthPending, subscribeToReauth } from "./session";
import { LoginPage } from "./pages/LoginPage";
import { SnippetList } from "./pages/SnippetList";
import { SnippetDetail } from "./pages/SnippetDetail";
import { SnippetForm } from "./pages/SnippetForm";
import { SettingsPage } from "./pages/SettingsPage";
import { ReorderPage } from "./pages/ReorderPage";
import { TempEditPage } from "./pages/TempEditPage";

type AuthState =
  | "checking"
  | "authenticated"
  | "unauthenticated"
  // The server didn't give a definite answer (down, restarting, 5xx).
  | "unreachable";

// How often to re-ask while the server is unreachable (it's usually just
// restarting).
const RETRY_INTERVAL_MS = 3000;

// React Router swaps out what's rendered based on the current URL,
// without a full page reload. Each <Route> maps a URL pattern to the
// component that should render for it.
function App() {
  // Gate the whole app behind its own login screen (not the browser's
  // native Basic Auth dialog) — on load, ask the server whether the
  // session cookie (if any) is still valid, and render the login form
  // instead of the app until it is.
  const [authState, setAuthState] = useState<AuthState>("checking");

  // Set by the API layer when a request finds the session has expired
  // mid-use (e.g. the server restarted). The login form then appears as an
  // overlay ON TOP of the app, so nothing the user was typing is lost.
  const reauthPending = useSyncExternalStore(subscribeToReauth, isReauthPending);

  const verifySession = useCallback(async () => {
    try {
      setAuthState((await checkAuth()) ? "authenticated" : "unauthenticated");
    } catch {
      setAuthState("unreachable");
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    checkAuth()
      .then((ok) => !cancelled && setAuthState(ok ? "authenticated" : "unauthenticated"))
      .catch(() => !cancelled && setAuthState("unreachable"));
    return () => {
      cancelled = true;
    };
  }, []);

  // While the server is unreachable, keep trying in the background.
  useEffect(() => {
    if (authState !== "unreachable") return;
    const timer = setInterval(verifySession, RETRY_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [authState, verifySession]);

  if (authState === "checking") {
    return <div className="app" />;
  }

  if (authState === "unreachable") {
    return (
      <div className="app">
        <div className="login-page">
          <div className="login-card">
            <h1>Snippet Memory</h1>
            <p className="login-subtitle">Can't reach the server.</p>
            <p className="error" role="alert">
              It may be restarting. Trying again automatically…
            </p>
            <button type="button" className="button" onClick={verifySession}>
              Try again now
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (authState === "unauthenticated") {
    return (
      <div className="app">
        <LoginPage onSuccess={() => setAuthState("authenticated")} />
      </div>
    );
  }

  return (
    <>
      <BrowserRouter>
        {/* inert: while the overlay is up, nothing underneath can be
            focused or clicked — but it all stays mounted. */}
        <div className="app" inert={reauthPending}>
          <Routes>
            <Route path="/" element={<SnippetList />} />
            <Route path="/snippets/new" element={<SnippetForm />} />
            <Route path="/snippets/:id" element={<SnippetDetail />} />
            <Route path="/snippets/:id/edit" element={<SnippetForm />} />
            <Route path="/snippets/:id/temp-edit" element={<TempEditPage />} />
            <Route
              path="/settings"
              element={<SettingsPage onLogout={() => setAuthState("unauthenticated")} />}
            />
            <Route path="/settings/reorder" element={<ReorderPage />} />
          </Routes>
        </div>
      </BrowserRouter>

      {reauthPending && (
        <div
          className="session-overlay"
          role="dialog"
          aria-modal="true"
          aria-label="Session expired"
        >
          <LoginPage expired onSuccess={completeReauth} />
        </div>
      )}
    </>
  );
}

export default App;
