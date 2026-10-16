package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestFromBuildInfo(t *testing.T) {
	vcs := []debug.BuildSetting{
		{Key: "vcs", Value: "git"},
		{Key: "vcs.revision", Value: "84054bd38974868a86161301bf06ce08dd369870"},
		{Key: "vcs.time", Value: "2026-10-16T12:05:07Z"},
		{Key: "vcs.modified", Value: "true"},
	}
	tests := []struct {
		name string
		bi   *debug.BuildInfo
		ok   bool
		want Info
	}{
		{
			name: "no build info",
			ok:   false,
			want: Info{Version: Devel},
		},
		{
			name: "dependency of another module",
			bi: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/app", Version: "v1.2.3"},
				Deps: []*debug.Module{{Path: ModulePath, Version: "v0.9.5"}},
			},
			ok:   true,
			want: Info{Version: "v0.9.5"},
		},
		{
			name: "dependency replaced by another version",
			bi: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/app"},
				Deps: []*debug.Module{{
					Path: ModulePath, Version: "v0.9.4",
					Replace: &debug.Module{Path: "example.com/fork", Version: "v0.9.4-fork.1"},
				}},
			},
			ok:   true,
			want: Info{Version: "v0.9.4-fork.1"},
		},
		{
			name: "dependency replaced by a local directory",
			bi: &debug.BuildInfo{
				Main: debug.Module{Path: "example.com/app"},
				Deps: []*debug.Module{{
					Path: ModulePath, Version: "v0.9.4",
					Replace: &debug.Module{Path: "../gxpdf"},
				}},
			},
			ok:   true,
			want: Info{Version: Devel},
		},
		{
			name: "main module built from a dirty checkout",
			bi: &debug.BuildInfo{
				Main:     debug.Module{Path: ModulePath, Version: "v0.9.6-0.20261016120507-84054bd38974+dirty"},
				Settings: vcs,
			},
			ok: true,
			want: Info{
				Version:  "v0.9.6-0.20261016120507-84054bd38974+dirty",
				Revision: "84054bd38974868a86161301bf06ce08dd369870",
				Time:     "2026-10-16T12:05:07Z",
				Modified: true,
			},
		},
		{
			name: "main module via go run or go test",
			bi:   &debug.BuildInfo{Main: debug.Module{Path: ModulePath, Version: Devel}},
			ok:   true,
			want: Info{Version: Devel},
		},
		{
			name: "module not linked into the binary",
			bi:   &debug.BuildInfo{Main: debug.Module{Path: "example.com/app"}},
			ok:   true,
			want: Info{Version: Devel},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromBuildInfo(tt.bi, tt.ok); got != tt.want {
				t.Errorf("fromBuildInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestRead checks Read against the real test binary, where gxpdf is the main
// module built by go test.
func TestRead(t *testing.T) {
	if got := Read(); got.Version == "" {
		t.Fatal("Read().Version is empty; want a version or (devel)")
	}
}
