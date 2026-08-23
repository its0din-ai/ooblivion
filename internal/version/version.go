// Package version holds build metadata injected at compile time via ldflags.
package version

var (
	Version   = "dev"
	Commit    = "dev"
	BuildDate = ""
)

func String() string {
	if Commit != "" && Commit != "dev" {
		return Commit
	}
	return Version
}
