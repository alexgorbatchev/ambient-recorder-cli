package plist

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"reflect"
	"testing"
)

func TestNativePlistRoundTrip(t *testing.T) {
	value := map[string]any{"Label": "voice & notes", "Enabled": true, "Args": []any{"microphone <&>", "路径"}, "Count": float64(63)}
	data, err := XML(value)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/usr/bin/plutil", "-convert", "json", "-o", "-", "-")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("parse native XML: %v %s", err, out)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil || !reflect.DeepEqual(got, value) {
		t.Fatalf("native round trip changed values: got=%v err=%v", got, err)
	}
}

func TestNativePlistRejectsInvalidValues(t *testing.T) {
	if _, err := XML(make(chan int)); err == nil {
		t.Fatal("accepted a non-serializable Go value")
	}
	for _, data := range []string{"{", "null", "[null]"} {
		if _, err := xmlFromJSON([]byte(data)); err == nil {
			t.Fatalf("accepted invalid plist input: %s", data)
		}
	}
}
