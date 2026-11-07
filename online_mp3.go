package main

import (
	"encoding/binary"
	"github.com/HyperparameterSI/golet/backend/audio"
	"github.com/hajimehoshi/go-mp3"
	"io"
)

// Suppress go-mp3's full-file seek-index scan on startup. Only build the
// index on explicit seek, using the same progressively filled backing file.
type onlineMP3 struct {
	d       *mp3.Decoder
	source  io.ReadSeeker
	frames  int64
	indexed bool
	buf     []byte
}

func decodeTrack(r io.ReadSeeker, t track) (audio.Decoder, error) {
	if t.Format != "MP3" || t.Release == "" {
		d, _, err := audio.NewDecoder(r)
		return d, err
	}
	d, err := mp3.NewDecoder(struct{ io.Reader }{r})
	if err != nil {
		return nil, err
	}
	return &onlineMP3{d: d, source: r, frames: int64(t.Seconds * float64(d.SampleRate()))}, nil
}
func (d *onlineMP3) SampleRate() int { return d.d.SampleRate() }
func (d *onlineMP3) Channels() int   { return 2 }
func (d *onlineMP3) Len() int64      { return d.frames }
func (d *onlineMP3) Read(dst []float32) (int, error) {
	size := len(dst) / 2 * 4
	if cap(d.buf) < size {
		d.buf = make([]byte, size)
	}
	n, err := io.ReadFull(d.d, d.buf[:size])
	n -= n % 4
	for i := 0; i < n/2; i++ {
		dst[i] = float32(int16(binary.LittleEndian.Uint16(d.buf[2*i:]))) / 32768
	}
	if n > 0 {
		return n / 2, nil
	}
	return 0, err
}
func (d *onlineMP3) SetPosition(frame int64) error {
	if !d.indexed {
		if _, e := d.source.Seek(0, io.SeekStart); e != nil {
			return e
		}
		indexed, e := mp3.NewDecoder(d.source)
		if e != nil {
			return e
		}
		d.d = indexed
		d.indexed = true
	}
	_, e := d.d.Seek(frame*4, io.SeekStart)
	return e
}
