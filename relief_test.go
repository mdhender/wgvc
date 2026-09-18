package wgvc

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestAssignReliefLogsPlaceholderWithoutChangingRelief(t *testing.T) {
	world := World{Provinces: []Province{{Relief: 0.25}}}
	var output bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	previousPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	assignRelief(&world)

	if world.Provinces[0].Relief != 0.25 {
		t.Fatalf("relief = %g after placeholder pass, want 0.25", world.Provinces[0].Relief)
	}
	message := output.String()
	if !strings.Contains(message, "relief pass is not implemented") || !strings.Contains(message, reliefIssueURL) {
		t.Fatalf("log output = %q, want not-implemented message pointing to %s", message, reliefIssueURL)
	}
}
