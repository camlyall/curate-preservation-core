package version

import (
	"runtime/debug"
	"strings"
	"time"
)

// Version returns the module version recorded by the Go linker.
// For a tagged build this is the tag (e.g. v1.0.2).
// For an un-tagged build it is the pseudo-version
// (e.g. v1.0.2-0.20250605-6d1e8239a3m).
func Version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "devel" // fallback for 'go run .' during local dev
}

// Commit returns the 12-char Git hash or "unknown".
func Commit() string { return buildSetting("vcs.revision") }

// BuildTime returns the commit time in RFC3339 or "unknown".
func BuildTime() string { return buildSetting("vcs.time") }

func buildSetting(key string) string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == key {
				return s.Value
			}
		}
	}
	return "unknown"
}

// Identifier produces something suitable for PREMIS, e.g.
// "Curate Preservation System version=v1.0.2-0.20250605-6d1e8239a3m".
func Identifier() string {
	v := strings.TrimPrefix(Version(), "v")
	ts := BuildTime()
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		ts = t.UTC().Format("20060102150405")
	}
	rev := Commit()
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return "Curate Preservation System version=" + v + "-" + ts + "-" + rev
}
