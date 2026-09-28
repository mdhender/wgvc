// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package wgvc

import (
	"github.com/maloquacious/semver"
)

var (
	version = semver.Version{
		Major:      0,
		Minor:      8,
		Patch:      6,
		PreRelease: "alpha",
		Build:      semver.Commit(),
	}
)

func Version() semver.Version {
	return version
}
