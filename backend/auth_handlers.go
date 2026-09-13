package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

const sessionCookieName = "snippet_memory_session"

// Request bodies for the auth endpoints are just a password (or two) —
// capping them small blocks a trivial DoS where someone posts a
// multi-gigabyte body to a public, unauthenticated endpoint like
// /api/login to burn server memory.
const maxAuthBodyBytes = 1024 // 1 KB

// authAPI bundles the auth-related dependencies, the same pattern as
// `api` in handlers.go.
type authAPI struct {
	passwords *PasswordStore
	sessions  *SessionStore
	limiter   *loginLimiter
}

// requireAuth wraps a handler so it only runs for requests carrying a
// valid session cookie; everything else gets a 401. This is the app's
// own login gate — no browser-native Basic Auth dialog involved. The
// frontend shows its own login screen when it gets a 401 from
// GET /api/auth/check.
func (a *authAPI) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || !a.sessions.Valid(cookie.Value) {
			writeError(w, http.StatusUnauthorized, "not logged in")
			return
		}
		next(w, r)
	}
}

// setSessionCookie is shared by login (set a real session) and logout
// (clear it by writing an already-expired cookie with the same name).
// Secure is set dynamically from the request rather than hardcoded:
// hardcoding it true would silently break login on the plain-HTTP setup
// this app uses by default (browsers refuse to send Secure cookies over
// non-HTTPS connections), while hardcoding it false would mean the
// cookie travels unprotected even when TLS *is* in use (directly via
// SNIPPET_MEMORY_TLS_CERT/KEY, or a reverse proxy in front).
func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

// isHTTPS reports whether the request arrived over TLS — either
// terminated directly by this server (r.TLS set), or by a reverse proxy
// in front of it that says so via X-Forwarded-Proto. Trusting that
// header from an untrusted client can only make this *more* cautious,
// never less: if someone spoofs it over a real plain-HTTP connection,
// the cookie just gets marked Secure and the browser refuses to send it
// back — breaking their own login attempt, not creating a vulnerability.
func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// login handles POST /api/login — the app's own login form submits here
// (not a browser auth dialog). Body: {"password": "..."}.
func (a *authAPI) login(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if ok, retryAfter := a.limiter.Allowed(ip); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
		writeError(w, http.StatusTooManyRequests,
			fmt.Sprintf("too many failed login attempts — try again in %s", formatDuration(retryAfter)))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !a.passwords.Verify(body.Password) {
		a.limiter.RecordFailure(ip)
		writeError(w, http.StatusUnauthorized, "incorrect password")
		return
	}
	a.limiter.RecordSuccess(ip)

	token, err := a.sessions.Create()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	setSessionCookie(w, r, token, int(sessionTTL.Seconds()))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// logout handles POST /api/logout.
func (a *authAPI) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		a.sessions.Delete(cookie.Value)
	}
	setSessionCookie(w, r, "", -1)
	writeJSON(w, http.StatusNoContent, nil)
}

// check handles GET /api/auth/check — the frontend calls this once on
// load to decide whether to show the login screen or the app. Wrapped
// with requireAuth in main.go, so just reaching this handler at all
// means the session was valid.
func (a *authAPI) check(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": true})
}

// changePassword handles POST /api/settings/password. Requires the
// current password to confirm it's really the logged-in user (the
// session cookie alone proves *a* login happened, not that this
// particular request has the password memorized — same principle as
// most "change password" flows). Body:
// {"currentPassword": "...", "newPassword": "..."}.
//
// On success, every session (including this one) is invalidated — so a
// cookie stolen before the password was changed stops working
// immediately instead of staying valid for up to 30 more days. The
// frontend responds by sending the user back to the login screen to
// sign in again with the new password.
func (a *authAPI) changePassword(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !a.passwords.Verify(body.CurrentPassword) {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}
	if len(body.NewPassword) < 4 {
		writeError(w, http.StatusBadRequest, "new password must be at least 4 characters")
		return
	}

	if err := a.passwords.Change(body.NewPassword); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save new password")
		return
	}
	a.sessions.Clear()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
