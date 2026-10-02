package resample

import (
	"math"
	"testing"
)

func TestStreamingConversion(t *testing.T) {
	for _, rate := range []int{16000, 44100, 48000, 96000} {
		for _, block := range []int{137, 8192} {
			c, err := New(rate)
			if err != nil {
				t.Fatal(err)
			}
			var output []float32
			for offset := 0; offset < rate; {
				n := min(block, rate-offset)
				pcm := make([]float32, n)
				for i := range pcm {
					pcm[i] = float32(0.5 * math.Sin(2*math.Pi*880*float64(offset+i)/float64(rate)))
				}
				converted, err := c.Convert(pcm)
				if err != nil {
					t.Fatal(err)
				}
				output = append(output, converted...)
				offset += n
			}
			tail, err := c.Finish()
			if err != nil {
				t.Fatal(err)
			}
			output = append(output, tail...)
			if err := c.Close(); err != nil {
				t.Fatal(err)
			}
			if len(output) != 48000 {
				t.Fatalf("rate=%d block=%d: got %d frames", rate, block, len(output))
			}
			var squaredError float64
			for i := 1000; i < len(output)-1000; i++ {
				want := 0.5 * math.Sin(2*math.Pi*880*float64(i)/48000)
				delta := float64(output[i]) - want
				squaredError += delta * delta
			}
			if rms := math.Sqrt(squaredError / float64(len(output)-2000)); rms > 0.01 {
				t.Fatalf("rate=%d block=%d: waveform error=%g", rate, block, rms)
			}
		}
	}
}

func TestConversionLifecycle(t *testing.T) {
	if c, err := New(0); err == nil {
		c.Close()
		t.Fatal("accepted zero sample rate")
	}
	c, err := New(16000)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Convert(make([]float32, 8193)); err == nil {
		t.Fatal("accepted oversized input")
	}
	if _, err := c.Finish(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Convert([]float32{1}); err == nil {
		t.Fatal("accepted input after EOF")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Convert([]float32{1}); err == nil {
		t.Fatal("accepted input after close")
	}
}
