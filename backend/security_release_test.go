//go:build release

package main

import "testing"

// A shipped binary must not trust the dev server's origin.
func TestReleaseBuildDoesNotTrustViteOrigin(t *testing.T) {
	h := mustBuild(t, stubMux())
	for _, sfs := range []string{"", "same-site"} {
		hdr := jsonH(map[string]string{"Origin": "http://localhost:5173"})
		if sfs != "" {
			hdr["Sec-Fetch-Site"] = sfs
		}
		if w := (secReq{method: "POST", host: "localhost:8080", body: "{}", headers: hdr}).do(h); w.Code != 403 {
			t.Errorf("vite origin (sfs=%q) = %d, want 403 in a release build", sfs, w.Code)
		}
	}
}
