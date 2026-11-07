package main

import (
	"io"
	"math"

	"github.com/HyperparameterSI/golet/backend/audio"
	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/frame"
)

type flacDecoder struct {
	*flac.Stream
	block  *frame.Frame
	offset int
}

func init() {
	audio.RegisterFormat("FLAC", "fLaC", func(r io.ReadSeeker) (audio.Decoder, error) {
		stream, err := flac.NewSeek(r)
		if err != nil {
			return nil, err
		}
		return &flacDecoder{Stream: stream}, nil
	})
}
func (d *flacDecoder) SampleRate() int { return int(d.Info.SampleRate) }
func (d *flacDecoder) Channels() int   { return int(d.Info.NChannels) }
func (d *flacDecoder) Len() int64 {
	if d.Info.NSamples == 0 {
		return -1
	}
	return int64(d.Info.NSamples)
}
func (d *flacDecoder) SetPosition(pos int64) error {
	actual, err := d.Seek(uint64(pos))
	if err == nil {
		d.block = nil
		d.offset = int(pos - int64(actual))
	}
	return err
}
func (d *flacDecoder) Read(dst []float32) (int, error) {
	n := 0
	ch := d.Channels()
	scale := float32(math.Ldexp(1, 1-int(d.Info.BitsPerSample)))
	for n+ch <= len(dst) {
		if d.block == nil {
			f, err := d.ParseNext()
			if err != nil {
				return n, err
			}
			d.block = f
		}
		for d.offset < len(d.block.Subframes[0].Samples) && n+ch <= len(dst) {
			for _, s := range d.block.Subframes {
				dst[n] = float32(s.Samples[d.offset]) * scale
				n++
			}
			d.offset++
		}
		if d.offset == len(d.block.Subframes[0].Samples) {
			d.block = nil
			d.offset = 0
		}
	}
	return n, nil
}
