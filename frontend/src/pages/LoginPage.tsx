import { useState } from "react";
import { login } from "../api";

interface LoginPageProps {
  onSuccess: () => void;
}

// The app's own login screen — shown instead of the browser's native
// Basic Auth dialog. App.tsx renders this whenever GET /api/auth/check
// comes back unauthenticated.
export function LoginPage({ onSuccess }: LoginPageProps) {
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      await login(password);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
      setSubmitting(false);
    }
  }

  return (
    <div className="login-page">
      <form onSubmit={handleSubmit} className="login-card">
        <h1>Snippet Memory</h1>
        <p className="login-subtitle">Enter the password to continue.</p>

        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          placeholder="Password"
          autoFocus
          className="login-input"
        />

        {error && <p className="error">{error}</p>}

        <button
          type="submit"
          className="button login-submit"
          disabled={submitting || !password}
        >
          {submitting ? "Checking..." : "Log in"}
        </button>
      </form>
    </div>
  );
}
