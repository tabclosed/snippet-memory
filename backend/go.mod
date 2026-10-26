module snippet-memory

// Keep this on a currently supported Go release: the Go team only patches
// the two newest major versions (https://go.dev/doc/devel/release), so
// anything older ships with known, unfixed standard-library CVEs
// (net/http, crypto/tls, ...). The release workflow reads this line to
// choose its toolchain, so this is the only place to change when bumping.
go 1.26
