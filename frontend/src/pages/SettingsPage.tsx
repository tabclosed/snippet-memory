import { useState } from "react";
import { Link } from "react-router-dom";
import { changePassword, logout } from "../api";

interface SettingsPageProps {
  onLogout: () => void;
}

const REPO_URL = "https://github.com/tabclosed/snippet-memory";

export function SettingsPage({ onLogout }: SettingsPageProps) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  async function handleChangePassword(e: React.FormEvent) {
    e.preventDefault();
    setPasswordError(null);

    if (newPassword !== confirmPassword) {
      setPasswordError("New password and confirmation don't match.");
      return;
    }

    setSaving(true);
    try {
      await changePassword(currentPassword, newPassword);
      // Changing the password invalidates every session, including this
      // one (so a stolen cookie stops working immediately) — so the
      // current login is now dead too. Send back to the login screen to
      // sign in again with the new password, rather than leaving the
      // user on a page that will start failing its next request.
      onLogout();
    } catch (err) {
      setPasswordError(
        err instanceof Error ? err.message : "Failed to change password"
      );
      setSaving(false);
    }
  }

  async function handleLogout() {
    await logout();
    onLogout();
  }

  return (
    <div className="page">
      <Link to="/" className="back-link">
        &larr; Back to snippets
      </Link>

      <div className="page-header">
        <h1>Settings</h1>
      </div>

      <section className="settings-section">
        <h2>Snippet order</h2>
        <p className="settings-section-hint">
          Change the order snippets appear in within each category.
        </p>
        <Link to="/settings/reorder" className="button button-secondary">
          Reorder Snippets
        </Link>
      </section>

      <section className="settings-section">
        <h2>Change password</h2>
        <form onSubmit={handleChangePassword} className="snippet-form">
          <label>
            Current password
            <input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
            />
          </label>

          <label>
            New password
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              minLength={4}
              required
            />
          </label>

          <label>
            Confirm new password
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              minLength={4}
              required
            />
          </label>

          {passwordError && <p className="error">{passwordError}</p>}

          <div className="button-group">
            <button type="submit" className="button" disabled={saving}>
              {saving ? "Saving..." : "Change Password"}
            </button>
          </div>
        </form>
      </section>

      <section className="settings-section">
        <h2>About</h2>
        <p className="settings-section-hint">
          This software is provided completely free and open source.
          Visit the{" "}
          <a href={REPO_URL} target="_blank" rel="noopener noreferrer">
            GitHub page
          </a>{" "}
          of this project to support and contribute.
        </p>
        <p className="settings-version">Version: 2026.09.10</p>
      </section>

      <section className="settings-section">
        <h2>Session</h2>
        <button
          type="button"
          className="button button-danger"
          onClick={handleLogout}
        >
          Log out
        </button>
      </section>
    </div>
  );
}
