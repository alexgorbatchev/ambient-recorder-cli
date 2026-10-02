package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/capture"
)

func TestMicrophoneListing(t *testing.T) {
	for _, agent := range []bool{false, true} {
		var out bytes.Buffer
		devices := []capture.Device{{ID: 17, UID: "stable-id", Name: "Office microphone", Transport: "USB", Type: "Microphone", Manufacturer: "Example Audio", Default: true}}
		if err := writeMicrophones(&out, devices, agent); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"Office microphone", "stable-id", "USB", "Example Audio"} {
			if !strings.Contains(out.String(), want) {
				t.Fatalf("listing missing %q: %s", want, out.String())
			}
		}
		if !agent && !strings.Contains(out.String(), "[system default]") {
			t.Fatal("default input not marked")
		}
		if agent && !strings.Contains(out.String(), "default=true") {
			t.Fatal("agent listing missing default input")
		}
	}
}
