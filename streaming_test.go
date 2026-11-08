package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HyperparameterSI/golet/backend/audio"
)

func firstPCM(t *testing.T, r io.ReadSeeker) {
	t.Helper()
	d, _, e := audio.NewDecoder(r)
	if e != nil {
		t.Fatal(e)
	}
	buf := make([]float32, d.Channels()*2048)
	if n, e := d.Read(buf); n != len(buf) || e != nil {
		t.Fatalf("first PCM: %d %v", n, e)
	}
}
func TestPlayBeforeDownloadCompletes(t *testing.T) {
	data := readAsset("prelude.flac")
	track := track{Size: int64(len(data)), Digest: fmt.Sprintf("%x", sha1.Sum(data))}
	r, w := io.Pipe()
	defer w.Close()
	path := filepath.Join(t.TempDir(), "track.flac")
	s, e := beginTransfer(r, track, path, new(transferProgress), func() { r.Close() })
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	split := 64 << 10
	if _, e = w.Write(data[:split]); e != nil {
		t.Fatal(e)
	}
	decoded := make(chan error, 1)
	go func() {
		d, _, e := audio.NewDecoder(s)
		if e == nil {
			_, e = d.Read(make([]float32, 4096))
		}
		decoded <- e
	}()
	select {
	case e := <-decoded:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("initial audio stalled waiting for EOF")
	}
	if _, e = os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("published incomplete track")
	}
	if _, e = w.Write(data[split:]); e != nil {
		t.Fatal(e)
	}
	w.Close()
	<-s.done
	if s.err != nil {
		t.Fatal(s.err)
	}
	s.Seek(-64, io.SeekEnd)
	tail := make([]byte, 64)
	if _, e = io.ReadFull(s, tail); e != nil || !bytes.Equal(tail, data[len(data)-64:]) {
		t.Fatal("cached seek failed", e)
	}
}
func TestTransferFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.flac")
	p := new(transferProgress)
	s, e := beginTransfer(io.NopCloser(bytes.NewReader([]byte("bad"))), track{Size: 3, Digest: "wrong"}, path, p, func() {})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	<-s.done
	if _, e = s.Read(make([]byte, 1)); e == nil || p.failure.Load() == nil {
		t.Fatal("ignored checksum failure")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 0 {
		t.Fatal("left invalid cache")
	}
}
func TestCancelledStreamReleasesBlockedRead(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	s, e := beginTransfer(r, track{Size: 200}, filepath.Join(t.TempDir(), "out"), new(transferProgress), func() { r.Close() })
	if e != nil {
		t.Fatal(e)
	}
	result := make(chan error, 1)
	go func() { _, e := s.Read(make([]byte, 1)); result <- e }()
	s.Close()
	select {
	case e := <-result:
		if e == nil {
			t.Fatal("reader missed cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("reader blocked")
	}
}

// Set OMNI_STARTUP_TEST=controlled or live to compare a cold full-file gate
// and progressive decoding with the same bundled public-domain FLAC recording.
func TestStartupComparison(t *testing.T) {
	mode := os.Getenv("OMNI_STARTUP_TEST")
	if mode == "" {
		t.Skip("opt-in timing test")
	}
	data := readAsset("prelude.flac")
	song := bundledPrelude(t)
	fetch := func() (io.ReadCloser, context.CancelFunc) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		if mode == "live" {
			req, _ := http.NewRequestWithContext(ctx, "GET", song.Source, nil)
			resp, e := http.DefaultClient.Do(req)
			if e != nil {
				cancel()
				t.Fatal(e)
			}
			if resp.StatusCode != 200 {
				resp.Body.Close()
				cancel()
				t.Fatal(resp.Status)
			}
			return resp.Body, cancel
		}
		r, w := io.Pipe()
		go func() {
			defer w.Close()
			time.Sleep(20 * time.Millisecond)
			for i := 0; i < len(data); i += 8192 {
				time.Sleep(12 * time.Millisecond)
				if _, e := w.Write(data[i:min(i+8192, len(data))]); e != nil {
					return
				}
			}
		}()
		return r, func() { r.Close(); cancel() }
	}
	start := time.Now()
	body, cancel := fetch()
	full, e := io.ReadAll(body)
	body.Close()
	cancel()
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha1.Sum(full)) != song.Digest {
		t.Fatal("incorrect fixture")
	}
	firstPCM(t, bytes.NewReader(full))
	baseline := time.Since(start)
	path := filepath.Join(t.TempDir(), "cold.flac")
	progress := new(transferProgress)
	start = time.Now()
	body, cancel = fetch()
	s, e := beginTransfer(body, song, path, progress, cancel)
	if e != nil {
		cancel()
		t.Fatal(e)
	}
	defer s.Close()
	firstPCM(t, s)
	elapsed := time.Since(start)
	t.Logf("%s: complete-file %v, first PCM %v, %.1fx; %d/%d bytes arrived", mode, baseline, elapsed, float64(baseline)/float64(elapsed), progress.bytes.Load(), song.Size)
}

func TestUnknownLengthTemporaryStream(t *testing.T) {
	data := []byte("temporary audio bytes")
	stream, err := beginTransfer(io.NopCloser(bytes.NewReader(data)), track{Size: -1, StreamOnly: true}, "", new(transferProgress), func() {})
	if err != nil {
		t.Fatal(err)
	}
	name := stream.file.Name()
	got, err := io.ReadAll(stream)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("unknown-length stream: %q %v", got, err)
	}
	if pos, err := stream.Seek(-5, io.SeekEnd); err != nil || pos != int64(len(data)-5) {
		t.Fatalf("seek: %d %v", pos, err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(name); !os.IsNotExist(err) {
		t.Fatal("temporary audio retained after close")
	}
}
