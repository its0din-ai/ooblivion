// Package version holds build metadata injected at compile time via ldflags.
package version

var (
	Version   = "dev"
	Commit    = "dev"
	BuildDate = ""
)

func String() string {
	switch {
	case Version != "" && Version != "dev":
		return Version
	case Commit != "" && Commit != "dev":
		return Commit
	}
	return "dev"
}
