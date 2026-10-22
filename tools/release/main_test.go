package main

import (
	"strings"
	"testing"
)

const testChangelog = `# Changelog

## [Unreleased]

## [1.3.0-rc.1] - 2026-10-20

### Added
- Release candidate

## [1.3.0] - 2026-10-22

### Added
- Something new

## [1.2.0]

## [0.9.0]
- Old
`

func TestPlan(t *testing.T) {
	tags := map[string]string{
		"v1.1.0":        "aaa",
		"v1.2.0":        "bbb",
		"v1.2.1-rc.1":   "ccc",
		"not-a-release": "ddd",
		"1.9.9":         "eee", // no "v": not a release tag
	}

	tests := []struct {
		name   string
		branch string
		commit string
		want   Plan
		err    string
	}{
		{name: "next version", branch: "release/1.3.0", commit: "fff",
			want: Plan{Version: "1.3.0", Tag: "v1.3.0"}},
		{name: "v prefix", branch: "release/v1.3.0", commit: "fff",
			want: Plan{Version: "1.3.0", Tag: "v1.3.0"}},
		{name: "prerelease", branch: "release/1.3.0-rc.1", commit: "fff",
			want: Plan{Version: "1.3.0-rc.1", Tag: "v1.3.0-rc.1", Prerelease: true}},
		{name: "an existing tag at this commit is still refused once a higher release exists", branch: "release/1.2.0", commit: "bbb",
			err: "not higher than the existing release v1.2.1-rc.1"},

		{name: "not a release branch", branch: "feature/1.3.0", commit: "fff", err: "not a release branch"},
		{name: "not semver", branch: "release/1.3", commit: "fff", err: "not a semantic version"},
		{name: "leading zero", branch: "release/1.03.0", commit: "fff", err: "not a semantic version"},
		{name: "build metadata", branch: "release/1.3.0+build", commit: "fff", err: "not a semantic version"},
		{name: "same version twice", branch: "release/1.2.0", commit: "fff", err: "v1.2.0 was already released"},
		{name: "lower than the current release", branch: "release/0.9.0", commit: "fff", err: "not higher than the existing release"},
		{name: "lower prerelease of the same version", branch: "release/1.2.1-rc.0", commit: "fff",
			err: "not higher than the existing release v1.2.1-rc.1"},
		{name: "no changelog section", branch: "release/2.0.0", commit: "fff", err: `no "## [2.0.0]" section`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := plan(tt.branch, tt.commit, tags, testChangelog)
			if tt.err != "" {
				if err == nil || !strings.Contains(err.Error(), tt.err) {
					t.Fatalf("plan() error = %v, want it to contain %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("plan() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("plan() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPlan_Resume(t *testing.T) {
	// An earlier run pushed v1.3.0 at this commit but did not finish publishing
	tags := map[string]string{"v1.2.0": "aaa", "v1.3.0": "fff"}

	got, err := plan("release/1.3.0", "fff", tags, testChangelog)
	if err != nil {
		t.Fatalf("plan() error = %v", err)
	}
	if want := (Plan{Version: "1.3.0", Tag: "v1.3.0", Resume: true}); got != want {
		t.Errorf("plan() = %+v, want %+v", got, want)
	}
}

func TestPlan_FirstRelease(t *testing.T) {
	got, err := plan("release/0.9.0", "fff", map[string]string{}, testChangelog)
	if err != nil {
		t.Fatalf("plan() error = %v", err)
	}
	if want := (Plan{Version: "0.9.0", Tag: "v0.9.0"}); got != want {
		t.Errorf("plan() = %+v, want %+v", got, want)
	}
}

func TestReleaseNotes(t *testing.T) {
	notes, err := releaseNotes(testChangelog, "1.3.0")
	if err != nil {
		t.Fatalf("releaseNotes() error = %v", err)
	}
	if want := "### Added\n- Something new"; notes != want {
		t.Errorf("releaseNotes() = %q, want %q", notes, want)
	}

	// An empty section is not a proper release
	if _, err := releaseNotes(testChangelog, "1.2.0"); err == nil || !strings.Contains(err.Error(), "is empty") {
		t.Errorf("releaseNotes(1.2.0) error = %v, want an empty-section error", err)
	}
	// "1.3.0" must not match the "1.3.0-rc.1" heading
	if notes, _ := releaseNotes(testChangelog, "1.3.0-rc.1"); notes != "### Added\n- Release candidate" {
		t.Errorf("releaseNotes(1.3.0-rc.1) = %q", notes)
	}
}

func TestCompare(t *testing.T) {
	// In ascending order, per the semver specification's precedence example
	ordered := []string{
		"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "1.10.0", "2.0.0",
	}
	for i := range ordered {
		for j := range ordered {
			a, err := parseVersion(ordered[i])
			if err != nil {
				t.Fatal(err)
			}
			b, err := parseVersion(ordered[j])
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			switch {
			case i < j:
				want = -1
			case i > j:
				want = 1
			}
			if got := compare(a, b); got != want {
				t.Errorf("compare(%s, %s) = %d, want %d", ordered[i], ordered[j], got, want)
			}
		}
	}
}
