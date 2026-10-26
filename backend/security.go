package main

import (
	"fmt"
	"log"
	"mime"
	"net/http"
	"strings"
)

// contentSecurityPolicy is deliberately strict about scripts — the built
// UI has no inline scripts and no eval — so an XSS bug (or a poisoned
// dependency that tries to load code from elsewhere) can't run
// attacker-supplied JavaScript or ship data to another host.
//
//   - script-src 'self'      only the bundled JS; no inline, no eval
//   - style-src ... 'unsafe-inline'
//     the code editor library injects a <style> element, so inline
//     styles have to be allowed. Style injection is far lower risk
//     than script injection.
//   - connect-src            the app's own API, plus GitHub's API: the
//     Settings page checks for a newer release from the browser.
//   - frame-ancestors 'none' nobody can embed the app (clickjacking).
//   - object-src/base-uri    close off plugin and <base> tag tricks.
//
// If you add a feature that loads something from another host (fonts,
// images, analytics...), it must be allowed here or the browser will
// block it and log a CSP violation in the console.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self' https://api.github.com; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// withSecurityHeaders adds browser-hardening headers to every response,
// including errors and redirects (it is the outermost wrapper).
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		// Never let the browser guess a different content type.
		h.Set("X-Content-Type-Options", "nosniff")
		// Legacy equivalent of CSP frame-ancestors, for old browsers.
		h.Set("X-Frame-Options", "DENY")
		// The Settings page links to GitHub; without this the browser
		// would tell GitHub the URL of your (possibly LAN) instance.
		h.Set("Referrer-Policy", "no-referrer")
		// Switch off powerful features the app never uses. Clipboard is
		// intentionally NOT listed: the copy buttons need it.
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")

		// Snippets can contain secrets: keep API responses out of the
		// browser cache and any shared proxy cache.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		// Only meaningful over HTTPS (browsers ignore it on plain HTTP).
		// No includeSubDomains/preload: this app doesn't own the domain.
		if isHTTPS(r) {
			h.Set("Strict-Transport-Security", "max-age=15552000")
		}
		next.ServeHTTP(w, r)
	})
}

// requireJSONBody rejects API writes whose body isn't declared as JSON.
//
// A cross-site <form> or a fetch() with mode "no-cors" can only send
// "simple" content types (text/plain, form-encoded, multipart) without a
// CORS preflight, and application/json is not one of them. Requiring it
// means those requests can't reach the handlers even if some browser
// doesn't send the headers the origin check relies on. The real frontend
// always sets this header on requests with a body; bodiless requests
// (logout, delete) are unaffected.
func requireJSONBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch:
				hasBody := r.ContentLength != 0 || len(r.TransferEncoding) > 0
				if hasBody {
					mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
					if err != nil || mt != "application/json" {
						writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
						return
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

// newCrossOriginProtection builds the CSRF defence. It uses the standard
// library's http.CrossOriginProtection, which, for every request that
// isn't a safe method (GET/HEAD/OPTIONS):
//
//   - trusts the browser's Sec-Fetch-Site header: only "same-origin" (or
//     "none", e.g. typed URL) passes. "same-site" is rejected too — that
//     is exactly the case SameSite=Lax cookies miss, where another app on
//     the same host but a different port can send your session cookie;
//   - when Sec-Fetch-Site is missing (older browsers), compares the
//     Origin header to Host;
//   - lets through requests with neither header — those aren't from a
//     browser, so there's no ambient cookie to abuse (curl, scripts).
//
// extraTrusted lists additional origins to allow (see main.go for the
// SNIPPET_MEMORY_TRUSTED_ORIGINS setting); dev builds also trust the Vite
// dev server (see devTrustedOrigins).
func newCrossOriginProtection(extraTrusted []string) (*http.CrossOriginProtection, error) {
	cop := http.NewCrossOriginProtection()
	for _, origin := range append(devTrustedOrigins(), extraTrusted...) {
		if err := cop.AddTrustedOrigin(origin); err != nil {
			return nil, fmt.Errorf("trusted origin %q: %w", origin, err)
		}
	}
	cop.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Logged so a reverse-proxy misconfiguration (Host header not
		// forwarded) is diagnosable instead of a silent 403.
		log.Printf("blocked cross-origin %s %s (Origin=%q Sec-Fetch-Site=%q Host=%q): %v",
			r.Method, r.URL.Path, r.Header.Get("Origin"), r.Header.Get("Sec-Fetch-Site"), r.Host,
			cop.Check(r))
		writeError(w, http.StatusForbidden, "cross-origin request blocked")
	}))
	return cop, nil
}

// parseTrustedOrigins splits a comma-separated list such as
// "https://snippets.example.com, https://notes.example.net:8443".
func parseTrustedOrigins(csv string) []string {
	var out []string
	for _, part := range strings.Split(csv, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// buildHandler assembles the full middleware chain around the router.
// Outermost first:
//
//	security headers  - on every response, errors included
//	origin check      - blocks cross-origin writes (CSRF)
//	JSON body check   - blocks "simple request" form posts to the API
//	CORS              - dev-only allowlist (no-op in release builds)
func buildHandler(mux http.Handler, extraTrustedOrigins []string) (http.Handler, error) {
	cop, err := newCrossOriginProtection(extraTrustedOrigins)
	if err != nil {
		return nil, err
	}
	h := withCORS(mux)
	h = requireJSONBody(h)
	h = cop.Handler(h)
	h = withSecurityHeaders(h)
	return h, nil
}
