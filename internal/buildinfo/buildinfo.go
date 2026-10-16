// Package buildinfo reports the gxpdf module version and VCS details the
// running binary was built with, as recorded by the Go toolchain.
//
// The toolchain is the single source of truth: it stamps the module version
// into every binary (runtime/debug.BuildInfo), so the reported version cannot
// drift from the release tag the way a hand-maintained constant does.
//
// What is reported (Go 1.24+):
//
//	gxpdf used as a dependency          the required version, e.g. v0.9.5
//	go build/install in the repo, tag   v0.9.5 (v0.9.5+dirty with local changes)
//	go build/install after a tag        a pseudo-version, e.g. v0.9.6-0.20261101093000-1a2b3c4d5e6f
//	go run, go test, -buildvcs=false    (devel)
//	local replace directive             (devel)
package buildinfo

import "runtime/debug"

// ModulePath is the module path of gxpdf.
const ModulePath = "github.com/coregx/gxpdf"

// Devel is reported when the toolchain recorded no version for the module.
const Devel = "(devel)"

// Info describes the gxpdf build.
type Info struct {
	// Version is the module version, a pseudo-version, or Devel.
	Version string
	// Revision, Time and Modified describe the VCS state at build time. They
	// are only known when gxpdf itself is the main module (built from a
	// checkout), not when it is a dependency or installed from the module
	// cache.
	Revision string
	Time     string
	Modified bool
}

// Read returns the build information of the running binary.
func Read() Info {
	return fromBuildInfo(debug.ReadBuildInfo())
}

// fromBuildInfo extracts gxpdf's build information from bi.
func fromBuildInfo(bi *debug.BuildInfo, ok bool) Info {
	info := Info{Version: Devel}
	if !ok || bi == nil {
		return info
	}

	if bi.Main.Path == ModulePath {
		info.Version = moduleVersion(&bi.Main)
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Revision = s.Value
			case "vcs.time":
				info.Time = s.Value
			case "vcs.modified":
				info.Modified = s.Value == "true"
			}
		}
		return info
	}

	for _, dep := range bi.Deps {
		if dep.Path == ModulePath {
			info.Version = moduleVersion(dep)
			break
		}
	}
	return info
}

// moduleVersion returns the version of the code that was actually built:
// the replacement's version when the module is replaced.
func moduleVersion(m *debug.Module) string {
	if m.Replace != nil {
		m = m.Replace
	}
	if m.Version == "" {
		return Devel
	}
	return m.Version
}
