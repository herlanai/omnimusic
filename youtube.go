package main

import (
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/HyperparameterSI/golet/backend"
	"github.com/HyperparameterSI/golet/backend/ui"
	"omnimusic/locales"
)

const youtube = "YouTube"

var videoCatalog = []track{
	{Title: "爱人错过", Artist: "告五人", Language: "zh", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=6D79CYTxvOM"},
	{Title: "小幸运", Artist: "田馥甄", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=_sQSXwdtxlY"},
	{Title: "怎么了", Artist: "周兴哲", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=Y2ge3KrdeWs"},
	{Title: "突然好想你", Artist: "五月天", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=GtDRcXtDg-4"},
	{Title: "小情歌", Artist: "苏打绿", Language: "zh", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=1205I77ovXo"},
	{Title: "如果可以", Artist: "韦礼安", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=8MG--WuNW1Y"},
	{Title: "光年之外", Artist: "邓紫棋", Language: "zh", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=T4SimnaiktU"},
	{Title: "失落沙洲（现场版）", Artist: "徐佳莹", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=BNdfKKHS8yo"},
	{Title: "十年", Artist: "陈奕迅", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=JM9rx_hN1Ko"},
	{Title: "宝贝", Artist: "张悬", Language: "zh", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=TsriGIW30po"},
	{Title: "倒带", Artist: "蔡依林", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=cB7DIIG0ykk"},
	{Title: "遇见", Artist: "孙燕姿", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=m4nu_F_9dWU"},
	{Title: "晴天", Artist: "周杰伦", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=DYptgVvkVLQ"},
	{Title: "勇气", Artist: "梁静茹", Language: "zh", Group: "现代流行", Format: youtube, Source: "https://www.youtube.com/watch?v=nDchQNPuA0k"},
	{Title: "Pumped Up Kicks", Artist: "Foster the People", Language: "en", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=SDTZ7iX4vTQ"},
	{Title: "Midnight City", Artist: "M83", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=dX3k_QDnzHE"},
	{Title: "Take a Walk", Artist: "Passion Pit", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=dZX6Q-Bj_xg"},
	{Title: "Little Talks", Artist: "Of Monsters and Men", Language: "en", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=KtDmEy7R_c8"},
	{Title: "Islands", Artist: "The xx", Language: "en", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=fENm7XJPzKY"},
	{Title: "Young Blood", Artist: "The Naked and Famous", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=WdO85Qf4Poc"},
	{Title: "Walking on a Dream", Artist: "Empire of the Sun", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=rXsQmkz1T7A"},
	{Title: "The Mother We Share", Artist: "CHVRCHES", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=-Uyoe-poyqs"},
	{Title: "Such Great Heights", Artist: "The Postal Service", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=0wrsZog8qXg"},
	{Title: "Lisztomania", Artist: "Phoenix", Language: "en", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=4BJDNw7o6so"},
	{Title: "Bulletproof", Artist: "La Roux", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=NB7Elur17Co"},
	{Title: "Dog Days Are Over", Artist: "Florence + the Machine", Language: "en", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=v3cgdlHzQDQ"},
	{Title: "The Less I Know the Better", Artist: "Tame Impala", Language: "en", Group: "独立流行", Format: youtube, Source: "https://www.youtube.com/watch?v=sBzrzS1Ag_g"},
	{Title: "Fireflies", Artist: "Owl City", Language: "en", Group: "电子流行", Format: youtube, Source: "https://www.youtube.com/watch?v=KWLGyeg74es"},
}

var videoID = regexp.MustCompile("^[A-Za-z0-9_-]{11}$")

func youtubeURL(input string) string {
	u, err := url.Parse(strings.TrimSpace(input))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
		return ""
	}
	var id string
	switch strings.ToLower(u.Host) {
	case "youtu.be":
		id = strings.TrimPrefix(u.Path, "/")
	case "www.youtube.com", "m.youtube.com", "music.youtube.com", "youtube.com":
		if u.Path == "/watch" {
			id = u.Query().Get("v")
		} else if strings.HasPrefix(u.Path, "/shorts/") {
			id = strings.TrimPrefix(u.Path, "/shorts/")
		}
	}
	if !videoID.MatchString(id) {
		return ""
	}
	return "https://www.youtube.com/watch?v=" + id
}

func (s *studio) youtubeHeader(c *ui.Context) {
	ui.Column(c).AlignSelf(ui.Stretch).Gap(8).Children(func() {
		ui.Row(c).WidthPercent(100).MaxWidth(680).AlignSelf(ui.Start).Gap(8).Children(func() {
			input := ui.TextInput(c.Key("video-link"), &s.videoLink).Grow(1).Placeholder(locales.VideoLink()).Label(locales.VideoLink())
			play := ui.Button(c, locales.Play()).Clicked()
			if input.Submitted() || play {
				source := youtubeURL(s.videoLink)
				s.invalidVideo = source == ""
				if source != "" {
					s.playVideo(source)
				}
			}
		})
		if s.invalidVideo {
			txt(c, locales.InvalidVideo(), 13, c.Theme().TextMuted)
		}
	})
}

const playerOrigin = "https://app.omnimusic.desktop/"

func embedDocument(source string) string {
	canonical := youtubeURL(source)
	if canonical == "" {
		return ""
	}
	id := strings.TrimPrefix(canonical, "https://www.youtube.com/watch?v=")
	return `<!doctype html><meta name="viewport" content="width=device-width, initial-scale=1"><style>html,body,iframe{width:100%;height:100%;margin:0;border:0;display:block;background:#111}</style><iframe id="player" title="YouTube" referrerpolicy="strict-origin-when-cross-origin" allow="autoplay; encrypted-media; fullscreen; picture-in-picture" allowfullscreen src="https://www.youtube.com/embed/` + id + `?playsinline=1&amp;autoplay=1&amp;enablejsapi=1&amp;origin=https%3A%2F%2Fapp.omnimusic.desktop"></iframe>
<script>
function onYouTubeIframeAPIReady() {
  new YT.Player("player", {events: {onStateChange: function(event) {
    if (event.data === YT.PlayerState.PLAYING) document.title = "playing:` + id + `";
  }}});
}
</script><script src="https://www.youtube.com/iframe_api"></script>`
}
func (s *studio) closeVideo() {
	if s.videoView != nil {
		s.videoView.Destroy()
		s.videoView = nil
	}
	s.miniVideo = false
	s.videoFromRecent = false
	s.videoAudibleSince = time.Time{}
	s.videoSource = ""
	s.videoError = ""
}
func (s *studio) playVideo(source string) {
	source = youtubeURL(source)
	doc := embedDocument(source)
	if doc == "" || s.page != youtube {
		return
	}
	if s.loading {
		s.toggle()
	}
	if s.player != nil && s.player.Playing() {
		s.player.Pause()
	}
	s.videoError = ""
	s.videoSource = source
	if s.window == nil {
		return
	}
	if s.videoView == nil {
		v, err := s.window.NewWebView(golet.WebViewOptions{Page: golet.PageOptions{Untrusted: true}})
		if err != nil {
			s.videoError = err.Error()
			return
		}
		s.videoView = v
		v.Page().OnPageTitleUpdated(func(e *golet.TitleEvent) {
			if s.videoView == v {
				s.videoPlaybackTitle(e.Title)
			}
		})
		v.Page().OnAudibleChanged(func(audible bool) {
			if s.videoView != v {
				return
			}
			s.videoAudibleSince = time.Time{}
			if audible {
				s.videoAudibleSince = time.Now()
			}
			if s.window != nil {
				s.window.Invalidate()
			}
		})
	}
	s.videoView.Page().LoadHTML(doc, playerOrigin)
}

func (s *studio) youtubePage(c *ui.Context, rows []track, height float32) {
	ui.Column(c).Height(height).Grow(1).MinWidth(0).Gap(12).Children(func() {
		s.youtubeHeader(c)
		listHeight := height - 52
		if s.invalidVideo {
			listHeight -= 32
		}
		if s.miniVideo && s.videoSource != "" {
			const width float32 = 356
			const videoHeight = width * 9 / 16
			title, artist := s.videoLabels()
			level, phase := float32(0), 0.
			if !s.videoAudibleSince.IsZero() && !c.Preferences().ReduceMotion && !c.Preferences().HighContrast {
				phase = time.Since(s.videoAudibleSince).Seconds()
				level = float32(.67 + .16*math.Sin(phase*3) + .10*math.Sin(phase*5.3))
				c.After(time.Second / 30)
			}
			s.playbackBackground(c, videoHeight+16, 16, level, phase).Padding(8).Gap(16).AlignItems(ui.Center).Children(func() {
				s.videoSurface(c, width)
				ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
					txt(c, title, 18, c.Theme().Text).Bold().MaxLines(2)
					if artist != "" {
						txt(c, artist, 14, c.Theme().TextMuted)
					}
					ui.Row(c).Gap(8).Children(func() {
						if action(c, "popout", locales.VideoPlayer(), c.Theme().Text, 34) {
							s.miniVideo = false
						}
						if action(c, "close", locales.CloseMini(), c.Theme().Text, 34) {
							s.closeVideo()
						}
					})
				})
			})
			listHeight -= videoHeight + 28
		}
		s.tracks(c, rows, listHeight)
	})
}

func (s *studio) videoPlaybackTitle(title string) {
	id, ok := strings.CutPrefix(title, "playing:")
	if ok && videoID.MatchString(id) && s.videoSource == "https://www.youtube.com/watch?v="+id {
		s.recordPlay(s.videoSource)
	}
}

func youtubeTrack(source string) track {
	for _, song := range videoCatalog {
		if song.Source == source {
			return song
		}
	}
	return track{Title: "YouTube · " + strings.TrimPrefix(source, "https://www.youtube.com/watch?v="), Artist: "YouTube Music", Format: youtube, Source: source}
}

func (s *studio) videoLabels() (string, string) {
	song := youtubeTrack(s.videoSource)
	return song.Title, song.Artist
}

func (s *studio) videoSurface(c *ui.Context, width float32) {
	if s.videoError != "" {
		ui.Column(c).Size(width, width*9/16).Justify(ui.Center).Children(func() {
			txt(c, locales.PlayFailure(s.videoError), 15, c.Theme().TextMuted)
			if ui.Button(c, locales.Play()).Clicked() {
				s.playVideo(s.videoSource)
			}
		})
	} else if s.videoView != nil {
		ui.WebView(c, s.videoView).Size(width, width*9/16).Radius(12)
	} else {
		ui.Box(c).Size(width, width*9/16)
	}
}

func (s *studio) youtubeListeningPage(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	title, artist := s.videoLabels()
	s.detailPage(c, locales.NowPlaying(), func() { s.closingPlayer = true; c.AnimationFrame() }, func() {
		width := min(w-80, (h-186)*16/9)
		ui.Column(c).Width(width).AlignSelf(ui.Center).Grow(1).Justify(ui.Center).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
				txt(c, title, min(float32(32), w*.032), t.Text).Bold()
				if artist != "" {
					txt(c, artist, 16, t.TextMuted)
				}
				ui.Box(c).Grow(1)
				if action(c, "mini", locales.MiniPlayer(), t.Text, 36) {
					s.videoFromRecent = false
					s.miniVideo = true
					c.AnimationFrame()
				}
			})
			s.videoSurface(c, width)
		})
	})
}
