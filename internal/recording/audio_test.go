package recording

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestDecodedSourceIsolation(t *testing.T) {
	decoder := buildSourceDecoder(t)
	for _, rate := range []int{16000, 44100, 48000} {
		r := newTrackRunner(t)
		s, err := r.newSource(rate, 2)
		if err != nil {
			t.Fatal(err)
		}
		start := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
		for offset := 0; offset < rate; offset += rate / 100 {
			pcm := make([]float32, rate/100*3)
			for frame := range rate / 100 {
				for channel, frequency := range []float64{440, 880, 1320} {
					pcm[frame*3+channel] = float32(0.25 * math.Sin(2*math.Pi*frequency*float64(offset+frame)/float64(rate)))
				}
			}
			if err := s.write(context.Background(), start.Add(frameDuration(int64(offset), rate)), pcm); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.close(context.Background()); err != nil {
			t.Fatal(err)
		}
		paths := r.out.paths()
		if err := r.out.close(); err != nil {
			t.Fatal(err)
		}
		for channel, path := range paths {
			pcm := decodeSource(t, decoder, path)
			if len(pcm) != 48000 {
				t.Fatalf("source %s has %d frames at input rate %d", path, len(pcm), rate)
			}
			for i, frequency := range []float64{440, 880, 1320} {
				amplitude := sourceAmplitude(pcm[4800:43200], frequency)
				if i == channel && amplitude < 0.15 {
					t.Fatalf("source %s lost its %.0f Hz tone: %f", path, frequency, amplitude)
				}
				if i != channel && amplitude > 0.025 {
					t.Fatalf("source %s contains another source's %.0f Hz tone: %f", path, frequency, amplitude)
				}
			}
		}
	}
}

func buildSourceDecoder(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(testdir.New(t), "decode-sources")
	cmd := exec.Command("clang", "../opus/testdata/decode.c", "-I../../.tmp/native/include/opus", "../../.tmp/native/lib/libopus.a", "-lm", "-o", binary)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile source decoder: %v %s", err, out)
	}
	return binary
}

func decodeSource(t *testing.T, decoder, path string) []float32 {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	frames := opusDuration(t, b)
	var packets [][]byte
	var packet []byte
	for len(b) > 0 {
		header := 27 + int(b[26])
		offset := header
		for _, size := range b[27:header] {
			packet = append(packet, b[offset:offset+int(size)]...)
			offset += int(size)
			if size < 255 {
				packets = append(packets, packet)
				packet = nil
			}
		}
		b = b[offset:]
	}
	if len(packets) < 3 || len(packet) != 0 || packets[0][9] != 1 {
		t.Fatal("source is not a complete mono Opus stream")
	}
	preSkip := int(binary.LittleEndian.Uint16(packets[0][10:12]))
	var input bytes.Buffer
	for _, packet := range packets[2:] {
		if err := binary.Write(&input, binary.LittleEndian, uint32(len(packet))); err != nil {
			t.Fatal(err)
		}
		input.Write(packet) // bytes.Buffer writes cannot fail.
	}
	cmd := exec.Command(decoder)
	cmd.Stdin = &input
	decoded, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded)/4 < preSkip+int(frames) {
		t.Fatal("source decoder returned fewer frames than its duration")
	}
	pcm := make([]float32, int(frames))
	for i := range pcm {
		pcm[i] = math.Float32frombits(binary.LittleEndian.Uint32(decoded[(preSkip+i)*4:]))
	}
	return pcm
}

func sourceAmplitude(pcm []float32, frequency float64) float64 {
	var sine, cosine float64
	for i, sample := range pcm {
		angle := 2 * math.Pi * frequency * float64(i) / 48000
		sine += float64(sample) * math.Sin(angle)
		cosine += float64(sample) * math.Cos(angle)
	}
	return 2 * math.Hypot(sine, cosine) / float64(len(pcm))
}
