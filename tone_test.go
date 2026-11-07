package main

import (
	"context"
	"math"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/HyperparameterSI/golet/backend/audio"
)

// Exercise the same FLAC and floating-point path as the desktop player.
func TestTonePlaybackModes(t *testing.T) {

	f, err := openTrack(context.Background(), bundledPrelude(t), new(transferProgress))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, _, err := audio.NewDecoder(f)
	if err != nil {
		t.Fatal(err)
	}
	var bypass atomic.Bool
	bypass.Store(true)
	tone := newToneDecoder(d, f, &bypass)
	sample := make([]float32, d.SampleRate()*d.Channels()/4)
	readAt := func(dec audio.Decoder) {
		t.Helper()
		if e := dec.SetPosition(int64(d.SampleRate() * 2)); e != nil {
			t.Fatal(e)
		}
		if n, e := dec.Read(sample); n != len(sample) || e != nil {
			t.Fatalf("read %d: %v", n, e)
		}
	}
	readAt(d)
	reference := slices.Clone(sample)
	readAt(tone)
	if !slices.Equal(sample, reference) {
		t.Fatal("bypass changed decoded samples")
	}
	bypass.Store(false)
	readAt(tone)
	if slices.Equal(sample, reference) {
		t.Fatal("tone processing had no effect")
	}
	for _, x := range sample {
		if math.IsNaN(float64(x)) || math.Abs(float64(x)) > 1 {
			t.Fatalf("invalid processed peak %v", x)
		}
	}
	bypass.Store(true)
	readAt(tone)
	tail := len(sample) / 2
	if !slices.Equal(sample[tail:], reference[tail:]) {
		t.Fatal("switch did not return to clean audio")
	}
}
