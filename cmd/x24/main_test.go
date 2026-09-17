package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWritesSVG(t *testing.T) {
	output := filepath.Join(t.TempDir(), "map.svg")
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-seed", "0x0123456789abcdef",
		"-provinces", "30",
		"-islands", "3",
		"-attractants", "2",
		"-output", output,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v; stderr = %s", err, stderr.String())
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(data), "<svg ") {
		t.Fatalf("output does not start with SVG element: %.40q", data)
	}
	if !strings.Contains(stdout.String(), "30 land, 3→") {
		t.Errorf("stdout = %q", stdout.String())
	}
}
