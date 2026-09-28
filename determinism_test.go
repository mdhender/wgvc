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
			want:   "b6a6952e0d989f6bb5206625f008133d8b576c79b48219a541267834f7c37b31",
		},
		{
			name:   "widescreen",
			config: Config{WorldSeed: 1, ProvinceCount: 500, IslandCount: 5, AspectRatio: AspectRatioWidescreen},
			want:   "bd12cafadcce6509142938f148784775fb5b61c6634fd69988562b23c536c177",
		},
		{
			name:   "portrait",
			config: Config{WorldSeed: 0xdeadbeef, ProvinceCount: 800, IslandCount: 8, AspectRatio: AspectRatioPortrait},
			want:   "e729448d02cf65c4b2374ce01f4b5804c1c26c343462cf5e2cfaf644ff901fc1",
		},
		{
			name:   "climate",
			config: Config{WorldSeed: 42, ProvinceCount: 600, IslandCount: 6, PolarIce: 0.2, PeakChill: 0.9},
			want:   "f82f11e7022169d6f323f5eab9bc9e3401f83f8bd9dcdb6804ab5e9f85909fc3",
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
