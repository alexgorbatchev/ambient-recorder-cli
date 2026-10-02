package opus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alexgorbatchev/ambient-recorder-cli/internal/testdir"
)

func TestRoundTrip(t *testing.T) {
	decoder := buildDecoder(t)
	for _, rate := range []int{16000, 48000, 44100} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			var out bytes.Buffer
			e, err := New(&out, Config{SampleRate: rate, Bitrate: 32000, Complexity: 2})
			if err != nil {
				t.Fatal(err)
			}
			pcm := make([]float32, rate)
			for i := range pcm {
				pcm[i] = float32(0.4 * math.Sin(2*math.Pi*440*float64(i)/float64(rate)))
			}
			for start := 0; start < len(pcm); start += rate / 50 {
				if err := e.Encode(pcm[start:min(start+rate/50, len(pcm))]); err != nil {
					t.Fatal(err)
				}
			}
			before := parsePages(t, out.Bytes())
			if len(before.packets) < 3 || before.eos {
				t.Fatal("stream did not flush playable packets before close")
			}
			if err := e.Close(); err != nil {
				t.Fatal(err)
			}
			pages := parsePages(t, out.Bytes())
			if !pages.eos || pages.granule-uint64(pages.preSkip) != 48000 {
				t.Fatalf("duration=%d, preskip=%d, EOS=%v", pages.granule, pages.preSkip, pages.eos)
			}
			decoded := decodePackets(t, decoder, pages.packets[2:])
			if len(decoded)%4 != 0 || len(decoded)/4 < int(pages.granule) {
				t.Fatalf("decoded %d bytes for granule %d", len(decoded), pages.granule)
			}
			var energy float64
			for i := int(pages.preSkip); i < int(pages.granule); i++ {
				v := math.Float32frombits(binary.LittleEndian.Uint32(decoded[i*4:]))
				energy += float64(v * v)
			}
			if rms := math.Sqrt(energy / 48000); rms < 0.1 || rms > 0.5 {
				t.Fatalf("decoded signal RMS=%f", rms)
			}
		})
	}
}

// Opt-in inspection never opens hardware or uploads recordings. Use it to check
// a file from a controlled live run, including a stream ended by SIGKILL.
func TestRecordedFile(t *testing.T) {
	path := os.Getenv("RECORDER_OPUS_FILE")
	if path == "" {
		t.Skip("set RECORDER_OPUS_FILE to inspect a live recording")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pages := parsePages(t, b)
	if os.Getenv("RECORDER_REQUIRE_EOS") == "1" && !pages.eos {
		t.Fatal("recording has no final EOS page")
	}
	decoded := decodePackets(t, buildDecoder(t), pages.packets[2:])
	const rate = 48000
	if len(decoded) < rate*4 {
		t.Fatal("recording contains less than one second of decodable audio")
	}
	var energy, tone float64
	for start := 0; start+rate*4 <= len(decoded); start += rate * 4 {
		var cosine, sine float64
		for i := range rate {
			v := float64(math.Float32frombits(binary.LittleEndian.Uint32(decoded[start+i*4:])))
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatal("decoder returned non-finite audio")
			}
			energy += v * v
			angle := 2 * math.Pi * 880 * float64(i) / rate
			cosine += v * math.Cos(angle)
			sine += v * math.Sin(angle)
		}
		tone = max(tone, 2*math.Hypot(cosine, sine)/rate)
	}
	t.Logf("bytes=%d duration=%.3fs EOS=%v decoded_frames=%d RMS=%.6f max_880Hz_amplitude=%.6f", len(b), float64(pages.granule-uint64(pages.preSkip))/rate, pages.eos, len(decoded)/4, math.Sqrt(energy/float64(len(decoded)/4)), tone)
	if os.Getenv("RECORDER_REQUIRE_TONE") == "1" && tone < 0.02 {
		t.Fatal("known playback tone is missing from recording")
	}
}

func decodePackets(t *testing.T, decoder string, source [][]byte) []byte {
	t.Helper()
	var packets bytes.Buffer
	for _, packet := range source {
		if err := binary.Write(&packets, binary.LittleEndian, uint32(len(packet))); err != nil {
			t.Fatal(err)
		}
		packets.Write(packet)
	}
	cmd := exec.Command(decoder)
	cmd.Stdin = &packets
	decoded, err := cmd.Output()
	if err != nil {
		t.Fatalf("native decode: %v", err)
	}
	return decoded
}

func TestEncoderWriteFailure(t *testing.T) {
	path := filepath.Join(testdir.New(t), "readonly")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	if e, err := New(f, Config{SampleRate: 48000, Bitrate: 32000, Complexity: 2}); err == nil {
		if err := e.Close(); err != nil {
			t.Log(err)
		}
		t.Fatal("expected header write failure")
	}
}

func TestClosedEncoder(t *testing.T) {
	var out bytes.Buffer
	e, err := New(&out, Config{SampleRate: 48000, Bitrate: 32000, Complexity: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
	if err := e.Encode([]float32{0}); err == nil {
		t.Fatal("encoded after close")
	}
	if err := e.Close(); err != nil {
		t.Fatal(err)
	}
}

type oggPages struct {
	packets [][]byte
	preSkip uint16
	granule uint64
	eos     bool
}

func parsePages(t *testing.T, b []byte) oggPages {
	t.Helper()
	var result oggPages
	var packet []byte
	for len(b) > 0 {
		if len(b) < 27 || string(b[:4]) != "OggS" {
			t.Fatal("invalid Ogg page header")
		}
		segments := int(b[26])
		header := 27 + segments
		if len(b) < header {
			t.Fatal("truncated segment table")
		}
		size := header
		for _, n := range b[27:header] {
			size += int(n)
		}
		if len(b) < size {
			t.Fatal("truncated Ogg page payload")
		}
		page := bytes.Clone(b[:size])
		wantCRC := binary.LittleEndian.Uint32(page[22:26])
		clear(page[22:26])
		var crc uint32
		for _, v := range page {
			crc ^= uint32(v) << 24
			for range 8 {
				if crc&0x80000000 != 0 {
					crc = crc<<1 ^ 0x04c11db7
				} else {
					crc <<= 1
				}
			}
		}
		if crc != wantCRC {
			t.Fatal("Ogg page CRC mismatch")
		}
		result.granule = binary.LittleEndian.Uint64(b[6:14])
		result.eos = b[5]&4 != 0
		offset := header
		for _, n := range b[27:header] {
			packet = append(packet, b[offset:offset+int(n)]...)
			offset += int(n)
			if n < 255 {
				result.packets = append(result.packets, packet)
				packet = nil
			}
		}
		b = b[size:]
	}
	if len(result.packets) < 2 || string(result.packets[0][:8]) != "OpusHead" || result.packets[0][9] != 1 {
		t.Fatal("missing mono Opus headers")
	}
	result.preSkip = binary.LittleEndian.Uint16(result.packets[0][10:12])
	return result
}

func buildDecoder(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(testdir.New(t), "decode")
	cmd := exec.Command("clang", "testdata/decode.c", "-I../../.tmp/native/include/opus", "../../.tmp/native/lib/libopus.a", "-lm", "-o", binary)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile native decoder: %v\n%s", err, out)
	}
	return binary
}
