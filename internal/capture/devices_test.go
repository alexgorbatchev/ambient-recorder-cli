package capture

import (
	"slices"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/microphone"
)

func TestMicrophonePreferences(t *testing.T) {
	devices := []Device{{ID: 1, UID: "mac", Name: "MacBook Pro Microphone"}, {ID: 2, UID: "headset", Name: "BM202", Default: true}, {ID: 3, UID: "display", Name: "LG UltraFine Display Audio"}}
	for _, tt := range []struct {
		name        string
		preferences []string
		want        []uint32
	}{
		{"default", nil, []uint32{2, 3, 1}},
		{"named", []string{"LG UltraFine Display Audio", "BM202", "*"}, []uint32{3, 2, 1}},
		{"stable UID", []string{"uid:mac"}, []uint32{1, 2, 3}},
		{"missing preferred", []string{"missing", "*"}, []uint32{2, 3, 1}},
		{"implicit fallback", []string{"BM202"}, []uint32{2, 3, 1}},
		{"duplicate preferences", []string{"BM202", "uid:headset", "*", "*"}, []uint32{2, 3, 1}},
		{"name prefix", []string{"LG Ultra*", "*"}, []uint32{3, 2, 1}},
		{"name suffix", []string{"*Microphone"}, []uint32{1, 2, 3}},
		{"name substring", []string{"*UltraFine*"}, []uint32{3, 2, 1}},
		{"multiple wildcards", []string{"L*Ultra*Audio"}, []uint32{3, 2, 1}},
		{"case sensitive", []string{"lg ultra*", "*"}, []uint32{2, 3, 1}},
		{"anchored pattern", []string{"Ultra*", "*"}, []uint32{2, 3, 1}},
		{"exclude default", []string{"!BM202", "*"}, []uint32{3, 1}},
		{"exclude name pattern", []string{"!LG Ultra*"}, []uint32{2, 1}},
		{"exclude after wildcard", []string{"*", "!LG Ultra*"}, []uint32{2, 1}},
		{"exclusion overrides preference", []string{"LG Ultra*", "!LG UltraFine Display Audio", "*"}, []uint32{2, 1}},
		{"exclude UID", []string{"!uid:headset"}, []uint32{3, 1}},
		{"UID remains exact", []string{"uid:ma*"}, []uint32{2, 3, 1}},
		{"exclude all", []string{"!*", "*"}, nil},
		{"only exclusions with implicit fallback", []string{"!BM202", "!LG*"}, []uint32{1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ordered := Prefer(devices, parsePreferences(t, tt.preferences))
			var got []uint32
			for _, device := range ordered {
				got = append(got, device.ID)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("preference order=%v, want %v", got, tt.want)
			}
			if devices[0].ID != 1 {
				t.Fatal("ordering modified caller inventory")
			}
		})
	}
}

func TestMicrophoneNamePatternCharacters(t *testing.T) {
	devices := []Device{
		{ID: 1, UID: "default", Name: "Other microphone", Default: true},
		{ID: 2, UID: "usb", Name: "Desk/Mic [USB] (v2)+?"},
		{ID: 3, UID: "unicode", Name: "会議室 Microphone"},
		{ID: 4, UID: "newline", Name: "Desk\nMic"},
		{ID: 5, UID: "backslash", Name: "Desk\\Mic"},
	}
	for _, tt := range []struct {
		pattern string
		want    uint32
	}{
		{"Desk*USB] (v2)+?", 2},
		{"Desk/Mic [USB]*", 2},
		{"会議*", 3},
		{"Desk*Mic", 5},
		{"Desk\\*", 5},
		{"Desk/Mic [USB] (v2)+?", 2},
		{"Desk/Mic .*", 1},
		{"[Other]*", 1},
	} {
		t.Run(tt.pattern, func(t *testing.T) {
			got := Prefer(devices, parsePreferences(t, []string{tt.pattern}))
			if len(got) != len(devices) || got[0].ID != tt.want {
				t.Fatalf("pattern %q ordered %v, want first ID %d", tt.pattern, got, tt.want)
			}
		})
	}
	got := Prefer(devices, parsePreferences(t, []string{"!Desk*", "*"}))
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("wildcard did not exclude slash/newline/backslash names: %v", got)
	}
}

func TestMicrophonePreferenceReconnection(t *testing.T) {
	preferences := parsePreferences(t, []string{"uid:headset", "*"})
	builtIn := Device{ID: 1, UID: "mac", Name: "MacBook Pro Microphone", Default: true}
	headset := Device{ID: 2, UID: "headset", Name: "BM202"}
	for _, devices := range [][]Device{{builtIn}, {builtIn, headset}, {builtIn}, {builtIn, {ID: 9, UID: "headset", Name: "Renamed headset"}}} {
		got := Prefer(devices, preferences)[0]
		want := devices[len(devices)-1]
		if got.ID != want.ID {
			t.Fatalf("selected %v, want %v", got, want)
		}
	}
}

func parsePreferences(t *testing.T, values []string) []microphone.Selector {
	t.Helper()
	selectors, err := microphone.Parse(values)
	if err != nil {
		t.Fatal(err)
	}
	return selectors
}
