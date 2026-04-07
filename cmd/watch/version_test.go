package main

import "testing"

func TestFormatVersion(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		revision string
		modified string
		want     string
	}{
		{name: "base only", base: "0.1.0-dev", want: "0.1.0-dev"},
		{name: "revision only", base: "0.1.0-dev", revision: "abcdef1", want: "0.1.0-dev (abcdef1)"},
		{name: "long revision shortened", base: "0.1.0-dev", revision: "abcdef123456", want: "0.1.0-dev (abcdef1)"},
		{name: "modified only", base: "0.1.0-dev", modified: "true", want: "0.1.0-dev (dirty)"},
		{name: "revision and modified", base: "0.1.0-dev", revision: "abcdef1", modified: "false", want: "0.1.0-dev (abcdef1, clean)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatVersion(tt.base, tt.revision, tt.modified)
			if got != tt.want {
				t.Fatalf("formatVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeModified(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "true", want: "dirty"},
		{in: "dirty", want: "dirty"},
		{in: "false", want: "clean"},
		{in: "clean", want: "clean"},
		{in: "", want: ""},
		{in: "weird", want: ""},
	}

	for _, tt := range tests {
		if got := normalizeModified(tt.in); got != tt.want {
			t.Fatalf("normalizeModified(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
