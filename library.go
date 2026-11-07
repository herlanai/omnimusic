package main

import (
	"embed"
	"omnimusic/locales"

	"encoding/json"

	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

//go:embed assets/*
var assets embed.FS

type track struct {
	Title, Artist, Composer, Group, Format, Source, Digest string
	Language, Album, Release, License                      string
	Seconds                                                float64
	Size                                                   int64
}
type signal struct {
	ID, Score, Comments, Mentions int
	Title, Group                  string
}
type settings struct {
	Local         []track
	Recent        []string
	Likes         map[string]bool
	Light         bool
	OriginalSound bool
	FrostedGlass  bool
}
type lyric struct {
	at   float64
	text string
}

var stamp = regexp.MustCompile(`\[(\d+):(\d{2}(?:\.\d+)?)\]`)

func lyrics(path string) []lyric {
	b, err := os.ReadFile(strings.TrimSuffix(path, filepath.Ext(path)) + ".lrc")
	if err != nil {
		return nil
	}
	return parseLRC(string(b))
}
func parseLRC(text string) []lyric {
	var out []lyric
	for _, line := range strings.Split(text, "\n") {
		matches := stamp.FindAllStringSubmatch(line, -1)
		words := strings.TrimSpace(stamp.ReplaceAllString(line, ""))
		for _, m := range matches {
			min, _ := strconv.Atoi(m[1])
			sec, _ := strconv.ParseFloat(m[2], 64)
			out = append(out, lyric{float64(min*60) + sec, words})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at < out[j].at })
	return out
}
func dataPath() (string, error) {
	d, e := os.UserConfigDir()
	return filepath.Join(d, "OmniMusic", "library.json"), e
}
func (s *studio) save() {
	p, e := dataPath()
	if e == nil {
		e = os.MkdirAll(filepath.Dir(p), 0700)
	}
	if e == nil {
		b, _ := json.Marshal(s.settings)
		e = os.WriteFile(p+".new", b, 0600)
		if e == nil {
			e = os.Rename(p+".new", p)
		}
	}
	if e != nil {
		s.message = func() string { return locales.SaveFailure(e.Error()) }
	}
}
func readAsset(name string) []byte {
	b, e := assets.ReadFile("assets/" + name)
	if e != nil {
		panic(e)
	}
	return b
}
func (s *studio) removeLocal(source string) {
	match := func(t track) bool { return t.Source == source }
	s.Local = slices.DeleteFunc(s.Local, match)
	s.Recent = slices.DeleteFunc(s.Recent, func(id string) bool { return id == source })
	s.queue = slices.DeleteFunc(s.queue, match)
	delete(s.Likes, source)
	s.save()
}

const recentPage = "最近播放"

func (s *studio) recordPlay(source string) {
	if source == "" || (len(s.Recent) > 0 && s.Recent[0] == source) {
		return
	}
	// A local track may have been removed while its file was opening.
	if youtubeURL(source) == "" && s.current.Digest == "" && !slices.ContainsFunc(s.Local, func(t track) bool { return t.Source == source }) {
		return
	}
	s.Recent = slices.DeleteFunc(s.Recent, func(id string) bool { return id == source })
	s.Recent = append([]string{source}, s.Recent[:min(len(s.Recent), 99)]...)
	s.save()
}

func (s *studio) recentTracks() []track {
	bySource := make(map[string]track, len(s.catalog)+len(s.Local))
	for _, tracks := range [][]track{s.catalog, s.Local} {
		for _, t := range tracks {
			bySource[t.Source] = t
		}
	}
	rows := make([]track, 0, len(s.Recent))
	for _, source := range s.Recent {
		if t, ok := bySource[source]; ok {
			rows = append(rows, t)
		} else if url := youtubeURL(source); url != "" {
			rows = append(rows, youtubeTrack(url))
		}
	}
	return rows
}

func (s *studio) list() []track {
	var base []track
	switch s.page {
	case recentPage:
		base = s.recentTracks()
	case youtube:
		base = s.videoMix
	case "每日精选":
		base = s.mix
	case "钢琴", "纯音乐":
		base = s.genreMix[s.page]
	case "全部音乐", "我的收藏":
		base = append(append([]track(nil), s.catalog...), s.Local...)
	case "本地音乐":
		base = s.Local
	default:
		base = s.catalog
	}
	var out []track
	q := strings.ToLower(strings.TrimSpace(s.query))
	for _, t := range base {
		if s.page == "我的收藏" && !s.Likes[t.Source] {
			continue
		}
		if s.page == "现代流行" && t.Release == "" {
			continue
		}
		if !strings.Contains(strings.ToLower(t.Title+t.Composer+t.artistLabel()+t.Group+topicName(t.Group)), q) {
			continue
		}
		out = append(out, t)
	}
	return out
}
