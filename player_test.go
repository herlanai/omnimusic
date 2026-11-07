package main

import (
	"context"
	"image/png"
	"omnimusic/locales"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/HyperparameterSI/golet/backend/audio"
	"github.com/HyperparameterSI/golet/backend/i18n"
	"github.com/HyperparameterSI/golet/backend/ui"
)

func bundledPrelude(t *testing.T) track {
	t.Helper()
	for _, tr := range newStudio().catalog {
		if tr.Digest == preludeDigest {
			return tr
		}
	}
	t.Fatal("missing FLAC fixture")
	return track{}
}
func TestOfflineRecording(t *testing.T) {
	f, err := openTrack(context.Background(), bundledPrelude(t), new(transferProgress))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, format, err := audio.NewDecoder(f)
	if err != nil || format != "FLAC" {
		t.Fatalf("format %s: %v", format, err)
	}
	buf := make([]float32, d.Channels()*1000)
	for _, pos := range []int64{0, int64(d.SampleRate()), 0} {
		if err = d.SetPosition(pos); err != nil {
			t.Fatal(err)
		}
		if n, e := d.Read(buf); e != nil || n != len(buf) {
			t.Fatalf("read %d %v", n, e)
		}
	}
}
func TestDailyAndLibraryViews(t *testing.T) {
	s := newStudio()
	s.Light = false
	day := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	s.daily(day)
	first := slices.Clone(s.mix)
	s.daily(day.AddDate(0, 0, 1))
	if slices.Equal(first, s.mix) {
		t.Fatal("daily order unchanged")
	}
	v := ui.NewTester(s.view, 1240, 790)
	v.Frame()
	if path := os.Getenv("OMNI_PREVIEW"); path != "" {
		f, e := os.Create(path)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, v.Image()); e != nil {
			t.Fatal(e)
		}
	}
	if e := v.Click(locales.AllMusic()); e != nil {
		t.Fatal(e)
	}
	if s.page != "全部音乐" || len(s.list()) != len(s.catalog)+len(s.Local) {
		t.Fatal("catalog navigation failed")
	}
}

func TestTimedLyricsInCompactWindow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/Song.lrc", []byte("[00:03.12][00:06.50]副歌\n[00:01.00]前奏\n"), 0600); err != nil {
		t.Fatal(err)
	}
	lines := lyrics(dir + "/Song.wav")
	if len(lines) != 3 || lines[0].text != "前奏" || lines[2].at != 6.5 {
		t.Fatalf("unexpected cues: %+v", lines)
	}
	s := newStudio()
	s.Light = true
	s.current = track{Title: "Song", Artist: "本地文件", Format: "WAV"}
	s.lines = []lyric{{0, "当前歌词"}}
	view := ui.NewTester(s.view, 820, 610)
	view.Frame()
	if !view.HasText("当前歌词") {
		t.Fatal("compact player missing current lyric")
	}
	for _, name := range []string{locales.Play(), locales.Volume(), "导入音乐"} {
		if name == "导入音乐" {
			name = locales.ImportMusic()
		}
		r, ok := view.Find(name)
		if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 821 || r.Y+r.H > 611 {
			t.Fatalf("control %s outside compact viewport: %+v", name, r)
		}
	}
}

func TestSwitchRecommendations(t *testing.T) {
	s := newStudio()
	v := ui.NewTester(s.view, 1240, 790)
	before := map[string]bool{}
	for _, track := range s.mix {
		before[track.Source] = true
	}
	queue := s.queue[0]
	if err := v.Click(locales.Refresh()); err != nil {
		t.Fatal(err)
	}
	if len(s.mix) != 12 || s.batch != 1 || queue != s.queue[0] {
		t.Fatal("batch or playback queue changed unexpectedly")
	}
	for _, track := range s.mix {
		if before[track.Source] {
			t.Fatal("repeated track", track.Title)
		}
		before[track.Source] = true
	}
	if s.evidence[s.signal.Group].strength <= 0 || s.signal.ID == 0 {
		t.Fatal("missing topic evidence")
	}
}

func TestPopDiscoveryAndGlassThemes(t *testing.T) {
	s := newStudio()
	for _, track := range s.mix {
		if track.Format == youtube {
			if youtubeURL(track.Source) == "" {
				t.Fatal("missing official video")
			}
			continue
		}
		if track.Release == "" || track.License == "" {
			t.Fatal("daily mix should contain licensed pop music")
		}
	}
	for _, genre := range []string{"钢琴", "纯音乐"} {
		s.page = genre
		if len(s.list()) != 12 {
			t.Fatal("missing category recommendations")
		}
		for _, song := range s.list() {
			if song.Release != "" || (song.Group == "钢琴") != (genre == "钢琴") {
				t.Fatal("wrong genre", song.Title)
			}
		}
	}
	s.page = "每日精选"
	s.Light = true
	s.FrostedGlass = true
	view := ui.NewTester(s.view, 1240, 790)
	view.Frame()
	if p := os.Getenv("OMNI_LIGHT_PREVIEW"); p != "" {
		f, e := os.Create(p)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		if e = png.Encode(f, view.Image()); e != nil {
			t.Fatal(e)
		}
	}
	if e := view.Click(locales.Lyrics()); e != nil {
		t.Fatal(e)
	}
	if !view.HasText(locales.Back()) {
		t.Fatal("lyrics screen missing")
	}
	s.serial = 3
	s.lyricResults <- lyricResult{id: 2, plain: "old lyrics"}
	s.poll()
	if s.plainLyrics != "" {
		t.Fatal("stale lyrics")
	}
	s.lyricResults <- lyricResult{id: 3, plain: "first verse\nsecond verse"}
	view.Frame()
	if !view.HasText("second verse") {
		t.Fatal("plain lyrics missing")
	}
}

func TestBilingualPlayerView(t *testing.T) {
	s := newStudio()
	previous := s.current
	for _, language := range []string{"zh-Hans", "en"} {
		t.Setenv("GOLET_LOCALE", language)
		view := ui.NewTester(s.view, 820, 610)
		view.SetTitleBar(ui.TitleBar{Height: 32, Right: 132})
		view.Frame()
		expected := locales.In(language)
		if !view.HasText(expected.Discover()) {
			t.Fatal("missing translated navigation", language)
		}
		if view.HasText(expected.Language()) {
			t.Fatal("settings leaked into toolbar")
		}
		if err := view.Click(expected.Settings()); err != nil {
			t.Fatal(err)
		}
		if view.HasText(expected.Discover()) || !view.HasText(expected.Appearance()) {
			t.Fatal("settings did not replace the library")
		}
		if err := view.Click(i18n.Autonym(language)); err != nil {
			t.Fatal(err)
		}
		options := view.Menu()
		if len(options) != 3 {
			t.Fatalf("language menu: %v", options)
		}
		view.CloseMenu()
		if err := view.Click(expected.Back()); err != nil {
			t.Fatal(err)
		}
		if s.settingsOpen {
			t.Fatal("settings stayed open")
		}
		if err := view.Click(expected.AllMusic()); err != nil {
			t.Fatal(err)
		}
		if s.current != previous || s.page != "全部音乐" {
			t.Fatal("language affected player or navigation")
		}
	}
	if got := locales.In("en").TrackCount(1); got != "1 track" {
		t.Fatal(got)
	}
	if got := locales.In("en").TrackCount(12); got != "12 tracks" {
		t.Fatal(got)
	}
}

func TestInAppCommentsAndImmersiveLyrics(t *testing.T) {
	s := newStudio()
	view := ui.NewTester(s.view, 1240, 790)
	if err := view.Click(locales.ReadComments()); err != nil {
		t.Fatal(err)
	}
	if s.commentStory != s.signal.ID {
		t.Fatal("opened wrong discussion")
	}
	replies := s.comments[s.commentStory]
	if len(replies) == 0 {
		if !view.HasText(locales.NoComments()) {
			t.Fatal("missing empty state")
		}
	} else if !view.HasText(replies[0].Author) {
		t.Fatal("missing author")
	}
	if err := view.Click(locales.Back()); err != nil {
		t.Fatal(err)
	}
	if s.commentStory != 0 {
		t.Fatal("comments did not close")
	}
	s.lines = []lyric{{0, "Current cue"}, {10, "Next cue"}}
	player := view
	if err := player.Click(locales.Lyrics()); err != nil {
		t.Fatal(err)
	}
	if view.HasText(locales.Discover()) {
		t.Fatal("sidebar still visible")
	}
	player.Advance(300 * time.Millisecond)
	if !player.HasText("Current cue") || !player.HasText("Next cue") {
		t.Fatal("window did not render shared lyrics")
	}
	for _, control := range []string{locales.Next(), locales.Volume(), locales.Seek()} {
		r, ok := player.Find(control)
		if !ok || r.X+r.W > 1241 || r.Y+r.H > 791 {
			t.Fatalf("control outside player: %s %+v", control, r)
		}
	}
}

func TestListClickStaysInLibrary(t *testing.T) {
	s := newStudio()
	song := track{Title: "List selection", Source: t.TempDir() + "/selection.wav", Format: "WAV"}
	s.Local = []track{song}
	s.page = "本地音乐"
	v := ui.NewTester(s.view, 820, 610)
	if err := v.Click(song.Title); err != nil {
		t.Fatal(err)
	}
	if s.current != song || s.lyricsOpen || !v.HasText(locales.Discover()) {
		t.Fatal("selecting the track should start it in the library")
	}
	serial := s.serial
	if err := v.Click(locales.OpenPlayer()); err != nil {
		t.Fatal(err)
	}
	if !s.lyricsOpen || s.serial != serial {
		t.Fatal("opening the transport title should only change the view")
	}
	s.cancel()
}

func TestReturnFromListeningPage(t *testing.T) {
	s := newStudio()
	s.page = "我的收藏"
	s.query = "music"
	current, serial := s.current, s.serial
	v := ui.NewTester(s.view, 820, 610)
	if err := v.Click(locales.OpenPlayer()); err != nil {
		t.Fatal(err)
	}
	if !s.lyricsOpen || v.HasText(locales.Discover()) {
		t.Fatal("track entry did not cover the library")
	}
	backY := func() float32 {
		r, ok := v.Find(locales.Back())
		if !ok {
			t.Fatal("missing back button during transition")
		}
		return r.Y
	}
	v.Advance(90 * time.Millisecond)
	entering := backY()
	v.Advance(300 * time.Millisecond)
	settled := backY()
	if entering <= settled {
		t.Fatal("player did not slide up on entry")
	}
	if err := v.Click(locales.Back()); err != nil {
		t.Fatal(err)
	}
	v.Advance(90 * time.Millisecond)
	if backY() <= settled {
		t.Fatal("player did not slide down on exit")
	}
	v.Advance(240 * time.Millisecond)
	if s.lyricsOpen || !v.HasText(locales.Discover()) || s.page != "我的收藏" || s.query != "music" || s.current != current || s.serial != serial {
		t.Fatal("return changed the library or playback")
	}
}

func TestSyncedLyricsMoveWithPlayback(t *testing.T) {
	s := newStudio()
	s.lines = []lyric{{0, "First cue"}, {2, "Second cue"}, {4, "Third cue"}}
	pos := 0.
	view := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { s.syncedLyrics(c, pos) })
	}, 360, 250)
	view.Advance(20 * time.Millisecond)
	cueY := func() float32 {
		r, ok := view.Find("Second cue")
		if !ok {
			t.Fatal("missing second cue")
		}
		return r.Y
	}
	before := cueY()
	pos = 2
	view.Frame()
	view.Advance(100 * time.Millisecond)
	during := cueY()
	view.Advance(400 * time.Millisecond)
	after := cueY()
	if !(before > during && during > after) {
		t.Fatalf("cue did not glide: %v %v %v", before, during, after)
	}
	pos = 0
	view.Frame()
	view.Advance(400 * time.Millisecond)
	if y := cueY(); y != before {
		t.Fatalf("backward seek landed at %v instead of %v", y, before)
	}
}

func TestYouTubeDiscovery(t *testing.T) {
	s := newStudio()
	v := ui.NewTester(s.view, 820, 610)
	if err := v.Click("YouTube Music"); err != nil {
		t.Fatal(err)
	}
	if !v.HasText(locales.VideoLink()) {
		if _, ok := v.Find(locales.VideoLink()); !ok {
			t.Fatal("missing video input")
		}
	}
	current := s.current
	selected := s.list()[0]
	if err := v.Click(selected.Title); err != nil {
		t.Fatal(err)
	}
	if s.videoSource != selected.Source || v.HasText(locales.Discover()) || !v.HasText(selected.Artist) {
		t.Fatal("missing YouTube detail page")
	}
	v.Advance(300 * time.Millisecond)
	if err := v.Click(locales.MiniPlayer()); err != nil {
		t.Fatal(err)
	}
	if !s.miniVideo || s.videoSource != selected.Source || !v.HasText(locales.Discover()) {
		t.Fatal("mini player should keep the video and show the list")
	}
	next := s.list()[1]
	if err := v.Click(next.Title); err != nil || !s.miniVideo || s.videoSource != next.Source {
		t.Fatalf("could not switch songs in mini player: %v", err)
	}
	if err := v.Click(locales.VideoPlayer()); err != nil || s.miniVideo || s.videoSource != next.Source {
		t.Fatalf("could not expand current video: %v", err)
	}
	v.Advance(300 * time.Millisecond)
	if err := v.Click(locales.Back()); err != nil {
		t.Fatal(err)
	}
	v.Advance(240 * time.Millisecond)
	if s.videoSource != "" || s.page != youtube || s.current != current || !v.HasText(locales.Discover()) {
		t.Fatal("did not return to YouTube list")
	}
	if len(s.list()) != 12 {
		t.Fatalf("unexpected YouTube catalog size %d", len(s.list()))
	}
	first := slices.Clone(s.list())
	if err := v.Click(locales.Refresh()); err != nil {
		t.Fatal(err)
	}
	assertFresh := func(rows []track) {
		t.Helper()
		seen := map[string]bool{}
		for _, song := range first {
			seen[song.Source] = true
		}
		if len(rows) != 12 {
			t.Fatalf("got %d videos", len(rows))
		}
		for _, song := range rows {
			if seen[song.Source] {
				t.Fatalf("repeated video %s", song.Title)
			}
			seen[song.Source] = true
		}
	}
	assertFresh(s.list())
	s.daily(time.Now().AddDate(0, 0, 1))
	assertFresh(s.list())
	for _, page := range []string{"全部音乐", "每日精选", "现代流行", "我的收藏", "钢琴", "纯音乐"} {
		s.page = page
		for _, tr := range s.list() {
			if tr.Format == youtube {
				t.Fatal("YouTube leaked onto", page)
			}
		}
	}
	for _, lang := range []string{"en", "zh-Hans"} {
		t.Setenv("GOLET_LOCALE", lang)
		s.daily(time.Now())
		if s.mix[0].Language != lang[:2] {
			t.Fatal("language preference ignored", lang, s.mix[0])
		}
	}
	id := "6D79CYTxvOM"
	for _, url := range []string{"https://youtu.be/" + id + "?si=1", "https://music.youtube.com/watch?v=" + id, "https://www.youtube.com/watch?v=" + id} {
		if !strings.Contains(embedDocument(url), "https://www.youtube.com/embed/"+id) || strings.Contains(embedDocument(url), "/watch?") || youtubeURL(url) != "https://www.youtube.com/watch?v="+id {
			t.Fatal(url)
		}
	}
	for _, url := range []string{"https://youtube.com.example/watch?v=" + id, "file:///watch?v=" + id, "https://youtu.be/bad", "https://youtube.com@evil.test/watch?v=" + id} {
		if youtubeURL(url) != "" {
			t.Fatal(url)
		}
	}
}

func TestRemoveLocalFromList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	item := track{Title: "Example", Source: dir + "/Example.mp3", Format: "MP3"}
	if err := os.WriteFile(item.Source, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	s := newStudio()
	s.page = "本地音乐"
	s.Local = []track{item}
	s.queue = []track{item, s.current}
	s.Likes[item.Source] = true
	view := ui.NewTester(s.view, 820, 610)
	if err := view.Click(locales.RemoveLocal(item.Title)); err != nil {
		t.Fatal(err)
	}
	if s.loading || len(s.queue) != 1 || !view.HasText(locales.ImportHint()) {
		t.Fatal("remove should clear the row without starting playback")
	}
	reopened := newStudio()
	if len(reopened.Local) != 0 || reopened.Likes[item.Source] {
		t.Fatal("removed track survived in the library")
	}
	if b, err := os.ReadFile(item.Source); err != nil || string(b) != "original" {
		t.Fatal("original file changed")
	}
}

func TestRecentPlaysPersistInPlaybackOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	s := newStudio()
	a, b := s.catalog[0], s.catalog[1]
	for _, song := range []track{a, b, a} {
		s.current = song
		s.recordPlay(song.Source)
	}
	reopened := newStudio()
	reopened.page = recentPage
	if rows := reopened.list(); !slices.Equal(rows, []track{a, b}) {
		t.Fatalf("unexpected recent plays: %v", rows)
	}
}

func TestRecentYouTubePlayback(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	s := newStudio()
	native := s.catalog[0]
	s.current = native
	s.recordPlay(native.Source)
	s.page = youtube
	s.playVideo("https://youtu.be/123456789ab")
	s.videoPlaybackTitle("playing:123456789ab")
	pasted := youtubeTrack(s.videoSource)
	known := videoCatalog[0]
	s.playVideo(known.Source)
	s.videoPlaybackTitle("playing:123456789ab")
	if len(s.Recent) != 2 {
		t.Fatal("stale player event recorded a video")
	}
	s.videoPlaybackTitle("playing:6D79CYTxvOM")
	s.videoPlaybackTitle("playing:6D79CYTxvOM")
	reopened := newStudio()
	reopened.page = recentPage
	rows := reopened.list()
	if !slices.Equal(rows, []track{known, pasted, native}) {
		t.Fatalf("unexpected mixed history: %v", rows)
	}
	reopened.playList(rows[0], rows)
	if reopened.page != youtube || reopened.videoSource != known.Source || !reopened.videoFromRecent || reopened.loading {
		t.Fatal("history entry did not open the embedded player")
	}
}
