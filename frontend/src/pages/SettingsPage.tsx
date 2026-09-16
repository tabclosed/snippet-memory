import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { changePassword, logout } from "../api";

interface SettingsPageProps {
  onLogout: () => void;
}

const REPO_URL = "https://github.com/tabclosed/snippet-memory";
const RELEASES_URL = "https://github.com/tabclosed/snippet-memory/releases";
const RELEASES_API_URL =
  "https://api.github.com/repos/tabclosed/snippet-memory/releases/latest";

// The version of the app actually running — bump this on release. Dates
// (YYYY.MM.DD) sort correctly either as strings or numerically, so this
// doubles as a simple, readable version scheme.
const APP_VERSION = "2026.09.14";

type UpdateStatus =
  | { state: "checking" }
  | { state: "up-to-date" }
  | { state: "update-available"; latest: string }
  | { state: "error" };

// Parses a "YYYY.MM.DD" (optionally "vYYYY.MM.DD") version string into
// comparable numbers. Returns null if it doesn't look like that shape —
// callers should treat that as "can't tell," not "definitely older."
function parseVersion(v: string): [number, number, number] | null {
  const match = v.trim().match(/^v?(\d+)\.(\d+)\.(\d+)$/i);
  if (!match) return null;
  return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function isNewer(candidate: string, current: string): boolean {
  const c = parseVersion(candidate);
  const base = parseVersion(current);
  if (!c || !base) return false;
  for (let i = 0; i < 3; i++) {
    if (c[i] !== base[i]) return c[i] > base[i];
  }
  return false;
}

export function SettingsPage({ onLogout }: SettingsPageProps) {
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordError, setPasswordError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [updateStatus, setUpdateStatus] = useState<UpdateStatus>({
    state: "checking",
  });

  useEffect(() => {
    let cancelled = false;

    fetch(RELEASES_API_URL)
      .then((res) => {
        if (!res.ok) throw new Error("request failed");
        return res.json();
      })
      .then((data: { tag_name?: string }) => {
        if (cancelled) return;
        const latest = (data.tag_name ?? "").trim();
        if (!latest) {
          setUpdateStatus({ state: "error" });
        } else if (isNewer(latest, APP_VERSION)) {
          setUpdateStatus({ state: "update-available", latest });
        } else {
          setUpdateStatus({ state: "up-to-date" });
        }
      })
      .catch(() => {
        if (!cancelled) setUpdateStatus({ state: "error" });
      });

    return () => {
      cancelled = true;
    };
  }, []);

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
        <p className="settings-version">App version: {APP_VERSION}</p>
        <p
          className={
            "settings-version-status" +
            (updateStatus.state === "update-available"
              ? " settings-version-status-alert"
              : "")
          }
        >
          {updateStatus.state === "checking" && <>└── checking for updates…</>}
          {updateStatus.state === "up-to-date" && <>└── up to date</>}
          {updateStatus.state === "error" && (
            <>└── couldn't check for updates</>
          )}
          {updateStatus.state === "update-available" && (
            <>
              └── update available ({updateStatus.latest}){" "}
              <a href={RELEASES_URL} target="_blank" rel="noopener noreferrer">
                visit GitHub
              </a>
            </>
          )}
        </p>
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
