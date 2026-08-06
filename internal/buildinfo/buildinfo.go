// Package buildinfo exposes metadata injected into release binaries.
package buildinfo

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// Info is the build metadata returned by the version command.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// Current returns the metadata embedded in the running binary.
func Current() Info {
	return Info{Version: version, Commit: commit, BuildDate: buildDate}
}
