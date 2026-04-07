package main

import (
	"runtime/debug"
	"strings"
)

// commit and dirty can be set at build time via -ldflags.
var (
	commit string
	dirty  string
)

type vcsInfo struct {
	revision string
	modified string
}

func versionString() string {
	rev := strings.TrimSpace(commit)
	mod := normalizeModified(strings.TrimSpace(dirty))

	if info, ok := readVCSInfo(); ok {
		if rev == "" {
			rev = info.revision
		}
		if mod == "" {
			mod = info.modified
		}
	}

	return formatVersion(version, rev, mod)
}

func readVCSInfo() (vcsInfo, bool) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return vcsInfo{}, false
	}

	var info vcsInfo
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.revision = shortenRevision(s.Value)
		case "vcs.modified":
			info.modified = normalizeModified(s.Value)
		}
	}

	if info.revision == "" && info.modified == "" {
		return vcsInfo{}, false
	}
	return info, true
}

func shortenRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) > 7 {
		return revision[:7]
	}
	return revision
}

func normalizeModified(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "dirty":
		return "dirty"
	case "false", "clean":
		return "clean"
	default:
		return ""
	}
}

func formatVersion(base, revision, modified string) string {
	base = strings.TrimSpace(base)
	revision = shortenRevision(revision)
	modified = normalizeModified(modified)

	switch {
	case revision == "" && modified == "":
		return base
	case revision != "" && modified == "":
		return base + " (" + revision + ")"
	case revision == "" && modified != "":
		return base + " (" + modified + ")"
	default:
		return base + " (" + revision + ", " + modified + ")"
	}
}
