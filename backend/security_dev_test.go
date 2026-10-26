//go:build !release

package main

import "testing"

// Dev builds trust the Vite dev server so `npm run dev` can write to the API.
func TestDevBuildTrustsViteOrigin(t *testing.T) {
	h := mustBuild(t, stubMux())
	for _, sfs := range []string{"", "same-site"} { // proxy-less direct call is "same-site" (different port)
		hdr := jsonH(map[string]string{"Origin": "http://localhost:5173"})
		if sfs != "" {
			hdr["Sec-Fetch-Site"] = sfs
		}
		if w := (secReq{method: "POST", host: "localhost:8080", body: "{}", headers: hdr}).do(h); w.Code != 200 {
			t.Errorf("vite origin (sfs=%q) = %d, want 200", sfs, w.Code)
		}
	}
	hdr := jsonH(map[string]string{"Origin": "http://localhost:6666", "Sec-Fetch-Site": "same-site"})
	if w := (secReq{method: "POST", host: "localhost:8080", body: "{}", headers: hdr}).do(h); w.Code != 403 {
		t.Errorf("other localhost port = %d, want 403", w.Code)
	}
}
