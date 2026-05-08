// Package buildinfo carries version metadata injected via -ldflags at build time.
package buildinfo

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
