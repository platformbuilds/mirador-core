package version

// The values below are intended to be set via -ldflags at build time.
// Defaults are empty for local dev.
var (
    Version    = ""
    CommitHash = ""
    BuildTime  = ""
)

