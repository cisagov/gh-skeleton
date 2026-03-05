package main

import (
	"regexp"
	"testing"
)

// semverRegex matches a valid semantic version string (e.g. "1.0.1" or
// "1.2.3-alpha.1+build.2").
var semverRegex = regexp.MustCompile(
	`^(?P<major>0|[1-9]\d*)\.(?P<minor>0|[1-9]\d*)\.(?P<patch>0|[1-9]\d*)` +
		`(?:-(?P<prerelease>[a-zA-Z0-9][a-zA-Z0-9.-]*))?` +
		`(?:\+(?P<buildmeta>[a-zA-Z0-9][a-zA-Z0-9.-]*))?$`,
)

func TestVersionNotEmpty(t *testing.T) {
	t.Parallel()

	if version == "" {
		t.Error("version constant must not be empty")
	}
}

func TestVersionSemver(t *testing.T) {
	t.Parallel()

	if !semverRegex.MatchString(version) {
		t.Errorf("version %q is not a valid semantic version", version)
	}
}
