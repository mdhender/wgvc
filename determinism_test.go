// Copyright (c) 2026 Michael D Henderson. All rights reserved.

package wgvc

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

// TestGenerateMatchesRecordedHash guards the cross-architecture determinism
// contract (issue #57): identical Config must produce a bit-identical World on
// every architecture. The hash covers every field of the world, so a new
// fusable multiply-add anywhere in the pipeline, which arm64 fuses and amd64
// does not, or a call into a math function with a per-architecture
// implementation, changes the hash on one architecture only.
//
// After an intentional change to the generator, print the new values with
// WGVC_PRINT_WORLD_HASH=1 and confirm they agree on both architectures; on
// Apple silicon, GOARCH=amd64 go test runs the amd64 binary under Rosetta.
func TestGenerateMatchesRecordedHash(t *testing.T) {
	cases := []struct {
		name   string
		config Config
		want   string
	}{
		{
			name:   "issue-57",
			config: Config{WorldSeed: 0x0123456789abcdef, ProvinceCount: 1500, IslandCount: 12},
			want:   "a28dec1180e269434e8b6692eedc29380aa4ae63287d315105ee1106caade7b2",
		},
		{
			name:   "widescreen",
			config: Config{WorldSeed: 1, ProvinceCount: 500, IslandCount: 5, AspectRatio: AspectRatioWidescreen},
			want:   "1ad6a498e182622d94a5633fd285f81ac4c8eb36942cdd8c1766e940e7f54306",
		},
		{
			name:   "portrait",
			config: Config{WorldSeed: 0xdeadbeef, ProvinceCount: 800, IslandCount: 8, AspectRatio: AspectRatioPortrait},
			want:   "f57e12b70c1d60c1320724cfa50c9dc2a4ccc29250c4a2103e13c5da8e029ea6",
		},
		{
			name:   "climate",
			config: Config{WorldSeed: 42, ProvinceCount: 600, IslandCount: 6, PolarIce: 0.2, PeakChill: 0.9},
			want:   "0ca00d937b1058c3f698dfb1878af9b0394ead61dfe5b1991f7b25d7c41e4d7a",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			world, err := Generate(c.config)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			got := worldHash(world)
			if os.Getenv("WGVC_PRINT_WORLD_HASH") == "1" {
				t.Logf("%s: %s", c.name, got)
			}
			if got != c.want {
				t.Errorf("world hash = %s, want %s; if the generator changed on purpose, rerun with WGVC_PRINT_WORLD_HASH=1 on both arm64 and amd64 and record the shared value", got, c.want)
			}
		})
	}
}

// worldHash is a SHA-256 over the printed form of every field in the world.
// The %+v verb prints floats in their shortest round-trip form, so any change
// in any bit changes the hash.
func worldHash(world World) string {
	hash := sha256.New()
	fmt.Fprintf(hash, "%+v", world)
	return fmt.Sprintf("%x", hash.Sum(nil))
}
