import { useEffect, useState } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { checkAuth } from "./api";
import { LoginPage } from "./pages/LoginPage";
import { SnippetList } from "./pages/SnippetList";
import { SnippetDetail } from "./pages/SnippetDetail";
import { SnippetForm } from "./pages/SnippetForm";
import { SettingsPage } from "./pages/SettingsPage";
import { ReorderPage } from "./pages/ReorderPage";
import { TempEditPage } from "./pages/TempEditPage";

type AuthState = "checking" | "authenticated" | "unauthenticated";

// React Router swaps out what's rendered based on the current URL,
// without a full page reload. Each <Route> maps a URL pattern to the
// component that should render for it.
function App() {
  // Gate the whole app behind its own login screen (not the browser's
  // native Basic Auth dialog) — on load, ask the server whether the
  // session cookie (if any) is still valid, and render the login form
  // instead of the app until it is.
  const [authState, setAuthState] = useState<AuthState>("checking");

  useEffect(() => {
    checkAuth().then((ok) => setAuthState(ok ? "authenticated" : "unauthenticated"));
  }, []);

  if (authState === "checking") {
    return <div className="app" />;
  }

  if (authState === "unauthenticated") {
    return (
      <div className="app">
        <LoginPage onSuccess={() => setAuthState("authenticated")} />
      </div>
    );
  }

  return (
    <BrowserRouter>
      <div className="app">
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
  );
}

export default App;
