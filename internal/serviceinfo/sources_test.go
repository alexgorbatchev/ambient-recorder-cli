package serviceinfo

import (
	"context"
	"reflect"
	"testing"
)

func TestSourceFileSnapshot(t *testing.T) {
	path := socketPath(t)
	s, err := Listen(path, Info{Version: "sources", CurrentFiles: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	want := []string{"/audio/mic-1.opus", "/audio/mic-2.opus", "/audio/playback.opus"}
	input := append([]string(nil), want...)
	s.SetCurrentFiles(input)
	input[0] = "/mutated.opus"
	info, err := Query(context.Background(), path)
	if err != nil || !reflect.DeepEqual(info.CurrentFiles, want) {
		t.Fatalf("source snapshot mutated: %+v %v", info, err)
	}
	s.SetCurrentFiles([]string{})
	info, err = Query(context.Background(), path)
	if err != nil || info.CurrentFiles == nil || len(info.CurrentFiles) != 0 {
		t.Fatalf("closed files were not published as an empty list: %+v %v", info, err)
	}
}
