package main

import (
	"context"
	"errors"
	"github.com/HyperparameterSI/golet/backend"
	"github.com/HyperparameterSI/golet/backend/audio"
	_ "github.com/HyperparameterSI/golet/backend/audio/mp3"
	_ "github.com/HyperparameterSI/golet/backend/audio/vorbis"
	"io"
	"net/http"
	"omnimusic/locales"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const preludeDigest = "8ef0e32ed0fa70b0d07642c467e7b886146a7ef0"
const badSignDigest = "e358bbe3721525f153ed6f2baaa750676b1c4dc1"

func openTrack(ctx context.Context, t track, progress *transferProgress) (io.ReadSeekCloser, error) {
	if t.Digest == "" && !t.Remote {
		return os.Open(t.Source)
	}
	var asset string
	switch {
	case !t.Remote && t.Digest == preludeDigest:
		asset = "assets/prelude.flac"
	case !t.Remote && t.Digest == badSignDigest:
		asset = "assets/bad-sign.mp3"
	}
	if asset != "" {
		f, e := assets.Open(asset)
		if e != nil {
			return nil, e
		}
		return f.(io.ReadSeekCloser), nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "OmniMusic", "audio")
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if t.StreamOnly {
		return openOnline(ctx, t, progress, "")
	}
	path := filepath.Join(dir, t.Digest+"."+strings.ToLower(t.Format))
	if f, e := os.Open(path); e == nil {
		st, e := f.Stat()
		if e == nil && st.Size() == t.Size {
			return f, nil
		}
		f.Close()
	}
	return openOnline(ctx, t, progress, path)
}
func openOnline(ctx context.Context, t track, progress *transferProgress, path string) (io.ReadSeekCloser, error) {
	ctx, cancel := context.WithCancel(ctx)
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, t.Source, nil)
	if e != nil {
		cancel()
		return nil, e
	}
	resp, e := (&http.Client{Timeout: 8 * time.Minute}).Do(req)
	if e != nil {
		cancel()
		return nil, e
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		cancel()
		return nil, errors.New(locales.SourceStatus(resp.StatusCode))
	}
	if t.StreamOnly {
		t.Size = resp.ContentLength
		if t.Size <= 0 {
			t.Size = -1
		}
	}
	stream, e := beginTransfer(resp.Body, t, path, progress, cancel)
	if e != nil {
		resp.Body.Close()
		cancel()
		return nil, e
	}
	return stream, nil
}

type loaded struct {
	id    int
	meter *toneDecoder
	p     *audio.Player
	lines []lyric
	err   error
}

func (s *studio) start(t track) {
	if s.cancel != nil {
		s.cancel()
	}
	if s.player != nil {
		s.player.Close()
		s.player = nil
		s.meter = nil
	}
	s.cancel = nil
	s.serial++
	serial := s.serial
	s.current = t
	s.loading = true
	s.message = locales.Loading
	s.lines = nil
	s.plainLyrics = ""
	s.lyricsStatus = ""
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	if t.Release != "" {
		s.lyricsStatus = "正在查找歌词…"
		go func() {
			r := fetchLyrics(ctx, t, serial)
			select {
			case s.lyricResults <- r:
			case <-ctx.Done():
			}
		}()
	}
	progress := new(transferProgress)
	s.progress = progress
	go func() {
		f, e := openTrack(ctx, t, progress)
		var meter *toneDecoder
		var p *audio.Player
		var lines []lyric
		if e == nil {
			var d audio.Decoder
			d, e = decodeTrack(f, t)
			if e == nil {
				meter = newToneDecoder(d, f, &s.bypass)
				p = audio.NewPlayer(meter)
			}
			if e != nil {
				f.Close()
			}
			if t.Digest == "" && !t.Remote {
				lines = lyrics(t.Source)
			}
		}
		select {
		case s.ready <- loaded{id: serial, meter: meter, p: p, lines: lines, err: e}:
		case <-ctx.Done():
			if p != nil {
				p.Close()
			}
		}
	}()
}
func (s *studio) poll() {
	if s.player != nil && s.progress != nil {
		if failure := s.progress.failure.Load(); failure != nil {
			s.serial++
			s.player.Close()
			s.player = nil
			s.meter = nil
			s.message = func() string { return locales.PlayFailure((*failure).Error()) }
			return
		}
	}
	for {
		select {
		case r := <-s.ready:
			if r.id != s.serial {
				if r.p != nil {
					r.p.Close()
				}
				continue
			}
			s.loading = false
			if r.err != nil {
				s.message = func() string { return locales.PlayFailure(r.err.Error()) }
				continue
			}
			s.message = nil
			s.player = r.p
			s.meter = r.meter
			if s.current.Release == "" {
				s.lines = r.lines
			}
			s.player.SetVolume(s.volume)
			s.player.OnEnd(func() {
				select {
				case s.ends <- r.id:
				default:
				}
			})
			s.resumePlayback()
		case l := <-s.lyricResults:
			if l.id == s.serial {
				s.lines = l.lines
				s.plainLyrics = l.plain
				s.lyricsStatus = l.status
			}
		case id := <-s.ends:
			if id == s.serial && s.progress.failure.Load() == nil {
				s.step(1)
			}
		default:
			return
		}
	}
}
func (s *studio) step(delta int) {
	if len(s.queue) == 0 {
		return
	}
	idx := 0
	for i, t := range s.queue {
		if t.Source == s.current.Source {
			idx = (i + delta + len(s.queue)) % len(s.queue)
			break
		}
	}
	s.start(s.queue[idx])
}
func (s *studio) resumePlayback() {
	s.player.Play()
	if s.player.Playing() {
		s.recordPlay(s.current.Source)
	}
}

func (s *studio) toggle() {
	if s.current.Source == "" {
		return
	}
	if s.loading {
		if s.cancel != nil {
			s.cancel()
		}
		s.serial++
		s.loading = false
		s.message = nil
		return
	}
	if s.player == nil {
		if s.current.Source != "" {
			s.start(s.current)
		}
		return
	}
	if s.player.Playing() {
		s.player.Pause()
	} else {
		s.resumePlayback()
	}
}
func (s *studio) importFiles() {
	paths, e := golet.Dialog.Open(golet.OpenDialogOptions{Title: locales.ImportDialog(), Multiple: true, Filters: []golet.FileFilter{{Name: locales.Music(), Extensions: []string{"flac", "wav", "mp3", "ogg"}}}})
	if e != nil {
		s.message = e.Error
		return
	}
	if len(paths) == 0 {
		return
	}
	known := map[string]bool{}
	for _, t := range s.Local {
		known[t.Source] = true
	}
	for _, p := range paths {
		if known[p] {
			continue
		}
		known[p] = true
		ext := filepath.Ext(p)
		s.Local = append(s.Local, track{Source: p, Title: strings.TrimSuffix(filepath.Base(p), ext), Artist: "本地文件", Group: "本地音乐", Format: strings.ToUpper(strings.TrimPrefix(ext, "."))})
	}
	s.page = "本地音乐"
	s.query = ""
	s.save()
}
