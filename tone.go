package main

import (
	"io"
	"math"
	"sync/atomic"

	"github.com/HyperparameterSI/golet/backend/audio"
)

// toneDecoder works at the recording's own sample rate, before Golet's mixer.
// Filter history belongs to the decoding goroutine; the UI only sets bypass.
type toneDecoder struct {
	audio.Decoder
	io.Closer
	level                             atomic.Uint32
	bypass                            *atomic.Bool
	low, upper                        []float64
	bassRate, airRate, blendStep, mix float64
}

func newToneDecoder(d audio.Decoder, file io.Closer, bypass *atomic.Bool) *toneDecoder {
	rate := float64(d.SampleRate())
	coef := func(hz float64) float64 { return -math.Expm1(-2 * math.Pi * hz / rate) }
	t := &toneDecoder{Decoder: d, Closer: file, bypass: bypass, low: make([]float64, d.Channels()), upper: make([]float64, d.Channels()), bassRate: coef(150), airRate: coef(6500), blendStep: 1 / (.025 * rate)}
	if !bypass.Load() {
		t.mix = 1
	}
	return t
}

func (t *toneDecoder) SetPosition(frame int64) error {
	if err := t.Decoder.SetPosition(frame); err != nil {
		return err
	}
	clear(t.low)
	clear(t.upper)
	return nil
}

func (t *toneDecoder) Read(dst []float32) (int, error) {
	n, err := t.Decoder.Read(dst)
	target := 1.
	if t.bypass.Load() {
		target = 0
	}
	var energy float64
	ch := len(t.low)
	for i := 0; i+ch <= n; i += ch {
		t.mix += max(-t.blendStep, min(t.blendStep, target-t.mix))
		for k := range ch {
			x := float64(dst[i+k])
			energy += x * x
			t.low[k] += t.bassRate * (x - t.low[k])
			t.upper[k] += t.airRate * (x - t.upper[k])
			if t.mix == 0 {
				continue
			}
			// Broad shelves, a small midrange trim, then a soft knee for rare peaks.
			wet := .94 * (x + .18*t.low[k] + .13*(x-t.upper[k]))
			if a := math.Abs(wet); a > .92 {
				wet = math.Copysign(.92+.08*math.Tanh((a-.92)/.08), wet)
			}
			dst[i+k] = float32(x + t.mix*(wet-x))
		}
	}
	if n > 0 {
		t.level.Store(math.Float32bits(float32(math.Sqrt(energy / float64(n)))))
	}
	return n, err
}
