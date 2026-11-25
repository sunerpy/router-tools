// Package buildinfo contains release metadata injected by the build.
package buildinfo

var (
	Version   = "dev"
	CommitID  = "unknown"
	BuildTime = "unknown"
)

// Info is safe to print in either CLI mode.
type Info struct {
	Version   string `json:"version"`
	CommitID  string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// Current returns the metadata for this binary.
func Current() Info {
	return Info{Version: Version, CommitID: CommitID, BuildTime: BuildTime}
}
