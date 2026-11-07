package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"omnimusic/locales"
	"sort"
	"strings"
	"time"

	"github.com/HyperparameterSI/golet/backend/ui"
)

type lyricResult struct {
	id            int
	lines         []lyric
	plain, status string
}

func fetchLyrics(ctx context.Context, t track, id int) lyricResult {
	r := lyricResult{id: id, status: "暂未收录这首歌的歌词"}
	params := url.Values{"track_name": {t.Title}, "artist_name": {t.Artist}, "album_name": {t.Album}, "duration": {fmt.Sprintf("%.0f", t.Seconds)}}
	req, e := http.NewRequestWithContext(ctx, "GET", "https://lrclib.net/api/get?"+params.Encode(), nil)
	if e != nil {
		return r
	}
	req.Header.Set("User-Agent", "OmniMusic/0.1.4")
	resp, e := (&http.Client{Timeout: 12 * time.Second}).Do(req)
	if e != nil {
		r.status = "歌词服务暂时无法连接"
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return r
	}
	if resp.StatusCode != 200 {
		r.status = "歌词服务暂时不可用"
		return r
	}
	var rec struct {
		SyncedLyrics, PlainLyrics string
		Instrumental              bool
	}
	if e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rec); e != nil {
		return r
	}
	r.lines = parseLRC(rec.SyncedLyrics)
	r.plain = strings.TrimSpace(rec.PlainLyrics)
	if len(r.lines) > 0 {
		r.status = "同步歌词 · LRCLIB"
	} else if r.plain != "" {
		r.status = "歌词 · LRCLIB"
	} else if rec.Instrumental {
		r.status = "纯音乐"
	}
	return r
}

func (s *studio) lyricsBody(c *ui.Context) {
	t := c.Theme()
	if len(s.lines) > 0 {
		pos := 0.
		if s.player != nil {
			pos = s.player.Position().Seconds()
		}
		s.syncedLyrics(c, pos)
	} else if s.plainLyrics != "" {
		ui.Scroll(c.Key(s.current.Source + "-text")).Grow(1).Children(func() {
			w, _ := c.Size()
			size := min(float32(34), max(float32(24), w*.025))
			ui.Column(c).Gap(14).Children(func() {
				for _, line := range strings.Split(s.plainLyrics, "\n") {
					txt(c, line, size, t.Text).Bold()
				}
			})
		})
	} else {
		s.lyricView(c)
	}
}

// syncedLyrics keeps the current cue near the upper third so there is room
// to read ahead. Each cue owns its animation in this window's Golet context.
func (s *studio) syncedLyrics(c *ui.Context, pos float64) {
	t := c.Theme()
	active := sort.Search(len(s.lines), func(i int) bool { return s.lines[i].at > pos }) - 1
	if active < 0 {
		txt(c, locales.AwaitVocals(), 13, t.TextMuted)
	}
	stage := ui.Box(c.Key(s.current.Source + "-cues")).Grow(1).MinHeight(0).AlignSelf(ui.Stretch).Clip()
	bounds := stage.Bounds()
	if bounds.W == 0 {
		c.After(time.Millisecond)
		return
	}
	size := min(float32(48), max(float32(30), bounds.W/14))
	gap := size * .38
	focus := max(0, active)
	start, end := max(0, focus-3), min(len(s.lines), focus+7)
	type cueLayout struct{ height, top float32 }
	var slots [10]cueLayout
	var y float32
	for i := start; i < end; i++ {
		_, height := c.MeasureText(max(1, bounds.W-6), ui.Span{Text: s.lines[i].text, Size: size, Weight: 700})
		height = max(size+4, height)
		slots[i-start] = cueLayout{height, y}
		y += height + gap
	}
	center := slots[focus-start]
	origin := max(0, (bounds.H-center.height)*.36) - center.top
	stage.Children(func() {
		for i := start; i < end; i++ {
			slot := slots[i-start]
			row := ui.Text(c.Key(i), s.lines[i].text).Absolute().Left(0).Right(6).FontSize(size).Bold()
			var strength float32
			if i == active {
				strength = 1
			}
			emphasis := row.AnimateWith("focus", strength, 300*time.Millisecond, ui.EaseInOut)
			base := t.TextMuted
			if i < active && !c.Preferences().HighContrast {
				base = base.Mix(t.Background, .25)
			}
			// Measure at full size so wrapping stays stable as focus changes.
			scale := .91 + .09*emphasis
			row.Top(row.AnimateWith("position", origin+slot.top, 380*time.Millisecond, ui.EaseOut)).
				TransformOrigin(0, .5).Scale(scale, scale).TextColor(base.Mix(s.accent(), emphasis))
		}
	})
}
