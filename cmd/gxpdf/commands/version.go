package commands

import (
	"fmt"
	"runtime"

	"github.com/coregx/gxpdf/internal/buildinfo"
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Long:  `Print the version, build date, and other information about gxpdf.`,
	Run: func(_ *cobra.Command, _ []string) {
		d := buildDetails(buildinfo.Read())
		fmt.Printf("gxpdf %s\n", d.version)
		fmt.Printf("  Go:         %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		if d.commit != "" {
			commit := d.commit
			if d.modified {
				commit += " (modified)"
			}
			fmt.Printf("  Commit:     %s\n", commit)
		}
		if d.date != "" {
			fmt.Printf("  Committed:  %s\n", d.date)
		}
	},
}

// versionDetails is the version information printed by the version command.
type versionDetails struct {
	version  string
	commit   string
	date     string
	modified bool
}

// buildDetails combines the release metadata injected by GoReleaser
// (-ldflags -X, see .goreleaser.yml) with what the Go toolchain recorded in
// the binary. Injected values win; without them (go install, go build from a
// checkout) the toolchain's module version and VCS details are used, so the
// output never falls back to a placeholder when the real values are known.
func buildDetails(tool buildinfo.Info) versionDetails {
	d := versionDetails{
		version:  tool.Version,
		commit:   tool.Revision,
		date:     tool.Time,
		modified: tool.Modified,
	}
	if Version != devVersion && Version != "" {
		d.version = canonicalVersion(Version)
	}
	if GitCommit != unknownValue && GitCommit != "" {
		d.commit = GitCommit
	}
	if BuildDate != unknownValue && BuildDate != "" {
		d.date = BuildDate
	}
	return d
}

// canonicalVersion returns v in Go's canonical "vMAJOR.MINOR.PATCH" form.
// GoReleaser's {{.Version}} drops the leading "v" while the toolchain keeps it;
// normalizing makes release binaries and go install report the same string.
func canonicalVersion(v string) string {
	if v != "" && v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}
