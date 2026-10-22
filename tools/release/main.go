// Command release enforces the release rules for the Release workflow.
//
//	go run ./tools/release check <branch> <commit>
//	go run ./tools/release notes <version>
//
// check reads a release branch name such as release/1.2.0, checks it against the
// repository's tags and CHANGELOG.md, and prints the version, tag, prerelease and
// resume values as key=value lines for $GITHUB_OUTPUT. notes prints the version's
// CHANGELOG.md section, which becomes the release notes.
//
// A release must be valid semver, higher than every existing release, never
// released before, and described in CHANGELOG.md.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const changelogFile = "CHANGELOG.md"

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

// run executes the command in args, writing its output to out.
func run(args []string, out io.Writer) error {
	changelog, err := os.ReadFile(changelogFile)
	if err != nil {
		return err
	}

	switch {
	case len(args) == 3 && args[0] == "check":
		tags, err := gitTags()
		if err != nil {
			return err
		}
		p, err := plan(args[1], args[2], tags, string(changelog))
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "version=%s\ntag=%s\nprerelease=%t\nresume=%t\n", p.Version, p.Tag, p.Prerelease, p.Resume)
		return nil
	case len(args) == 2 && args[0] == "notes":
		notes, err := releaseNotes(string(changelog), strings.TrimPrefix(args[1], "v"))
		if err != nil {
			return err
		}
		fmt.Fprintln(out, notes)
		return nil
	default:
		return errors.New("usage: release check <branch> <commit> | release notes <version>")
	}
}

// Plan is a release the rules allow.
type Plan struct {
	Version    string // e.g. 1.2.0
	Tag        string // e.g. v1.2.0
	Prerelease bool   // the version has a prerelease part, e.g. 1.2.0-rc.1
	// Resume means the tag already points at this commit: an earlier run pushed
	// it but did not finish publishing, so this run continues that release
	Resume bool
}

// plan checks a release from branch at commit against the existing tags (tag
// name to commit) and the changelog.
func plan(branch, commit string, tags map[string]string, changelog string) (Plan, error) {
	name, ok := strings.CutPrefix(branch, "release/")
	if !ok {
		return Plan{}, fmt.Errorf("%q is not a release branch; name it release/<version>, e.g. release/1.2.0", branch)
	}
	version, err := parseVersion(name)
	if err != nil {
		return Plan{}, fmt.Errorf("release branch %q: %w", branch, err)
	}
	p := Plan{Version: version.String(), Tag: "v" + version.String(), Prerelease: version.pre != ""}

	if at, exists := tags[p.Tag]; exists {
		if at != commit {
			return Plan{}, fmt.Errorf("%s was already released (from commit %.12s); versions are never released twice, so use a higher one", p.Tag, at)
		}
		p.Resume = true
	}

	for tag := range tags {
		if tag == p.Tag {
			continue
		}
		existing, err := parseVersion(tag)
		if err != nil || !strings.HasPrefix(tag, "v") {
			continue // not a release tag
		}
		if compare(version, existing) <= 0 {
			return Plan{}, fmt.Errorf("%s is not higher than the existing release %s; releases only move forward", p.Tag, tag)
		}
	}

	if _, err := releaseNotes(changelog, p.Version); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// releaseNotes returns the body of the "## [version]" section of the changelog.
func releaseNotes(changelog, version string) (string, error) {
	heading := "## [" + version + "]"
	var body []string
	found := false
	for line := range strings.SplitSeq(changelog, "\n") {
		if found {
			if strings.HasPrefix(line, "## ") {
				break
			}
			body = append(body, line)
			continue
		}
		// The heading may carry a date: "## [1.2.0] - 2026-10-22"
		if line == heading || strings.HasPrefix(line, heading+" ") {
			found = true
		}
	}
	notes := strings.TrimSpace(strings.Join(body, "\n"))
	if !found {
		return "", fmt.Errorf("%s has no %q section; move the [Unreleased] entries under it before releasing", changelogFile, heading)
	}
	if notes == "" {
		return "", fmt.Errorf("the %q section of %s is empty", heading, changelogFile)
	}
	return notes, nil
}

// gitTags returns every tag in the repository with the commit it points at.
func gitTags() (map[string]string, error) {
	// %(*objectname) is the commit of an annotated tag; it is empty for a
	// lightweight tag, whose %(objectname) is already the commit
	out, err := exec.Command("git", "for-each-ref", "--format=%(refname:strip=2) %(objectname) %(*objectname)", "refs/tags").Output()
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	tags := map[string]string{}
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		tags[fields[0]] = fields[len(fields)-1]
	}
	return tags, nil
}

// version is a semantic version (https://semver.org) without build metadata.
type version struct {
	major, minor, patch uint64
	pre                 string // prerelease identifiers, e.g. "rc.1"
}

var semver = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)` +
	`(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?$`)

// parseVersion parses "1.2.0", "v1.2.0" or "1.2.0-rc.1".
func parseVersion(s string) (version, error) {
	m := semver.FindStringSubmatch(s)
	if m == nil {
		return version{}, fmt.Errorf("%q is not a semantic version like 1.2.0 or 1.2.0-rc.1", s)
	}
	var v version
	for i, field := range []*uint64{&v.major, &v.minor, &v.patch} {
		n, err := strconv.ParseUint(m[i+1], 10, 64)
		if err != nil {
			return version{}, fmt.Errorf("%q: %w", s, err)
		}
		*field = n
	}
	v.pre = m[4]
	return v, nil
}

func (v version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
	if v.pre != "" {
		s += "-" + v.pre
	}
	return s
}

// compare returns -1, 0 or 1 as a is lower than, equal to or higher than b, by
// semver precedence: 1.0.0-alpha < 1.0.0-alpha.1 < 1.0.0-beta < 1.0.0-rc.1 < 1.0.0.
func compare(a, b version) int {
	for _, pair := range [][2]uint64{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}

	// A version without a prerelease part is higher than one with
	switch {
	case a.pre == b.pre:
		return 0
	case a.pre == "":
		return 1
	case b.pre == "":
		return -1
	}

	as, bs := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		if c := compareIdentifier(as[i], bs[i]); c != 0 {
			return c
		}
	}
	// All shared identifiers are equal, so the longer prerelease is higher
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}

// compareIdentifier compares prerelease identifiers: numeric ones by value, and
// lower than alphanumeric ones, which compare as strings.
func compareIdentifier(a, b string) int {
	an, aErr := strconv.ParseUint(a, 10, 64)
	bn, bErr := strconv.ParseUint(b, 10, 64)
	switch {
	case aErr == nil && bErr == nil:
		switch {
		case an < bn:
			return -1
		case an > bn:
			return 1
		}
		return 0
	case aErr == nil:
		return -1
	case bErr == nil:
		return 1
	}
	return strings.Compare(a, b)
}
