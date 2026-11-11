package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/HyperparameterSI/golet/backend"
	"github.com/HyperparameterSI/golet/backend/audio"
	"github.com/HyperparameterSI/golet/backend/i18n"
	"github.com/HyperparameterSI/golet/backend/plugins/glass"
	"github.com/HyperparameterSI/golet/backend/ui"
	"hash/fnv"
	"math"
	"omnimusic/locales"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

type studio struct {
	tray                                     *golet.Tray
	trayState                                string
	quitting                                 bool
	remotePools                              map[string][]track
	remotePending                            map[string]bool
	remoteWorkers                            map[string]int
	remoteResults                            chan remoteResult
	remoteContext                            context.Context
	remoteCancel                             context.CancelFunc
	remoteLoading                            map[string]bool
	recommendationPath                       string
	recommendationErrors                     map[string]string
	recommendationTime                       time.Time
	videoView                                *golet.WebView
	videoLink, videoSource, mixLanguage      string
	videoError                               string
	videoMix                                 []track
	videoBatch                               int
	invalidVideo, miniVideo, videoFromRecent bool
	videoAudibleSince                        time.Time
	settingsOpen                             bool
	comments                                 map[int][]hnComment
	commentStory, commentLimit               int
	window                                   *golet.Window
	pop                                      []track
	genreMix                                 map[string][]track
	genreBatch                               map[string]int
	lyricsOpen, closingPlayer                bool
	plainLyrics, lyricsStatus                string
	lyricResults                             chan lyricResult
	settings
	bypass              atomic.Bool
	catalog, mix, queue []track
	signals             []signal
	signal              signal
	batch               int
	evidence            map[string]topicEvidence
	date, page, query   string
	message             func() string
	current             track
	meter               *toneDecoder
	player              *audio.Player
	lines               []lyric
	volume              float64
	ready               chan loaded
	ends                chan int
	cancel              context.CancelFunc
	serial              int
	loading             bool
	progress            *transferProgress
	images              map[string]*ui.Bitmap
}

func newStudio() *studio {
	p, err := dataPath()
	if err != nil {
		// An invalid configuration path is reported by the recommendation view.
		return newStudioWithRemote("", true)
	}
	return newStudioWithRemote(filepath.Join(filepath.Dir(p), "recommendations.sqlite"), true)
}

func newStudioWithCache(path string) *studio { return newStudioWithRemote(path, false) }
func newStudioWithRemote(path string, remote bool) *studio {
	s := &studio{recommendationPath: path, recommendationErrors: map[string]string{}, settings: settings{Light: true, FrostedGlass: true}, lyricResults: make(chan lyricResult, 4), page: "每日精选", volume: .8, ready: make(chan loaded, 4), ends: make(chan int, 4), images: map[string]*ui.Bitmap{}}
	for name, dst := range map[string]any{"catalog.json": &s.catalog, "signals.json": &s.signals, "pop.json": &s.pop, "comments.json": &s.comments} {
		if e := json.Unmarshal(readAsset(name), dst); e != nil {
			panic(e)
		}
	}
	if p, e := dataPath(); e == nil {
		b, _ := os.ReadFile(p)
		_ = json.Unmarshal(b, &s.settings)
	}
	s.catalog = append(s.catalog, s.pop...)
	s.bypass.Store(s.OriginalSound)
	if s.Likes == nil {
		s.Likes = map[string]bool{}
	}
	for _, n := range append([]string{"record.png", "ribbon.jpg", "icon.png"}, sleeves()...) {
		b, e := ui.DecodeBitmap(readAsset(n))
		if e != nil {
			panic(e)
		}
		s.images[n] = b
	}
	s.evidence = summarize(s.signals)
	if remote {
		s.remoteContext, s.remoteCancel = context.WithCancel(context.Background())
		s.remoteResults = make(chan remoteResult, 8)
		s.remoteLoading = map[string]bool{}
		s.remotePools = map[string][]track{}
		s.remotePending = map[string]bool{}
		s.remoteWorkers = map[string]int{}
		s.catalog = nil
		s.restoreRemoteCatalog()
		s.restoreRemoteInventory()
	}
	s.daily(time.Now())
	for _, t := range s.catalog {
		if t.Digest == badSignDigest {
			s.current = t
			break
		}
	}
	if s.current.Source == "" && len(s.mix) > 0 {
		s.current = s.mix[0]
	}
	s.lyricsStatus = "播放后载入歌词"
	s.queue = []track{s.current}
	for _, t := range s.mix {
		if t.Source != s.current.Source {
			s.queue = append(s.queue, t)
		}
	}
	return s
}
func sleeves() []string {
	var names []string
	for i := 0; i < 10; i++ {
		names = append(names, fmt.Sprintf("sleeve%d.jpg", i))
	}
	return names
}
func cover(t track) string {
	h := fnv.New32a()
	h.Write([]byte(t.Composer + t.Title))
	return fmt.Sprintf("sleeve%d.jpg", h.Sum32()%10)
}
func clockText(v float64) string { sec := int(v); return fmt.Sprintf("%d:%02d", sec/60, sec%60) }
func txt(c *ui.Context, v string, size float32, col ui.Color) ui.Element {
	return ui.Text(c, v).FontSize(size).TextColor(col)
}

var hex = ui.Hex

func purpleGlass(dark bool) *ui.Theme {
	t := glass.Theme(dark)
	t.Accent = hex("#663c94")
	t.AccentHover = hex("#7849a8")
	t.AccentPressed = hex("#503074")
	t.Focus = t.Accent
	t.Selection = hex("#e9dff3")
	if dark {
		t.Background = hex("#171419")
		t.Surface = hex("#29262b")
		t.SurfaceHover = hex("#353139")
		t.SurfacePressed = hex("#403744")
		t.Border = hex("#454047")
		t.Accent = hex("#7848a3")
		t.Focus = hex("#ab7fd2")
		t.Selection = hex("#513565")
	}
	t.AccentMaterial = glass.Glass{Style: glass.Vivid, Tint: t.Accent, Interactive: true}
	return t
}

var lightGlass = func() *ui.Theme {
	t := purpleGlass(false)
	white := ui.RGB(255, 255, 255)
	t.Background = hex("#faf9f7")
	t.Surface = white
	t.SurfaceHover = hex("#f6f4f5")
	t.SurfacePressed = hex("#eee8ed")
	t.Text = hex("#242127")
	t.TextMuted = hex("#645e66")
	t.Border = hex("#e5e1e3")
	t.InputBorder = t.Border
	t.PanelMaterial = lightPaneGlass{}
	t.ControlMaterial = glass.Glass{Style: glass.Regular, Tint: white.Alpha(.45), Interactive: true}
	return t
}()

// Keep Golet’s backdrop blur and refraction, clipping its one-pixel
// directional rim outside the visible edge of large light panels.
type lightPaneGlass struct{}

func (lightPaneGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	pad := 3 / p.Scale()
	p.Clip(box, radii[0], func() {
		lens := ui.Rect{X: box.X - pad, Y: box.Y - pad, W: box.W + 2*pad, H: box.H + 2*pad}
		for i := range radii {
			radii[i] += pad
		}
		glass.Glass{Clarity: .6}.PaintMaterial(p, lens, radii)
		// Composite white after refraction so even a dark desktop yields white glass.
		p.Fill(box, ui.RGBA(255, 255, 255, .94), 0)
	})
}

var darkGlass = purpleGlass(true)
var lightSolid = opaqueTheme(lightGlass)
var darkSolid = opaqueTheme(darkGlass)

func opaqueTheme(t *ui.Theme) *ui.Theme {
	solid := *t
	solid.PanelMaterial, solid.AccentMaterial, solid.ControlMaterial = nil, nil, nil
	return &solid
}

// Custom panes follow the native controls' high-contrast fallback.
func glassPane(c *ui.Context, e ui.Element) ui.Element {
	if !c.Theme().Dark && c.Theme().PanelMaterial == nil && !c.Preferences().HighContrast {
		return e.Background(c.Theme().Surface)
	}
	if c.Theme().PanelMaterial == nil || c.Preferences().HighContrast {
		return e.Background(c.Theme().Surface).Border(1, c.Theme().Border)
	}
	return e.Material(c.Theme().PanelMaterial)
}

// Add reflected color over the surface, below its text and controls.
func surfaceWash(c *ui.Context, e ui.Element, from, to ui.Color, angle, radius float32) ui.Element {
	if !c.Preferences().HighContrast {
		e.Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillGradient(r, ui.LinearGradient{From: from, To: to, Angle: angle}, radius)
		})
	}
	return e
}

func (s *studio) color(d, l string) ui.Color {
	if s.Light {
		return hex(l)
	}
	return hex(d)
}
func (s *studio) accent() ui.Color { return s.color("#cbb3ff", "#663c94") }
func (s *studio) playList(t track, rows []track) {
	if t.Format == youtube {
		if s.page == recentPage {
			s.videoFromRecent = true
			s.page = youtube
		}
		s.playVideo(t.Source)
		return
	}
	s.queue = s.queue[:0]
	for _, row := range rows {
		if row.Format != youtube {
			s.queue = append(s.queue, row)
		}
	}
	s.start(t)
}
func (s *studio) like() {
	k := s.current.Source
	if k == "" {
		return
	}
	if s.Likes[k] {
		delete(s.Likes, k)
	} else {
		s.Likes[k] = true
	}
	s.save()
}
func (s *studio) nav(c *ui.Context, name, glyph string) {
	r := ui.Row(c.Key(name)).Height(40).AlignItems(ui.Center).Padding(0, 8).Gap(10).Radius(10).Focusable().Role(ui.RoleButton).Label(topicName(name))
	if name == s.page {
		if !s.FrostedGlass || c.Preferences().HighContrast {
			r.Background(c.Theme().SurfacePressed)
		} else {
			r.Material(glass.Glass{Style: glass.Vivid, Tint: s.accent().Alpha(.22), Interactive: true})
		}
	} else if r.Hovered() {
		r.Background(c.Theme().SurfaceHover)
	}
	col := c.Theme().TextMuted
	if name == s.page {
		col = s.accent()
	}
	r.Children(func() { icon(c, glyph, col, 17); txt(c, topicName(name), 14, col) })
	if r.Clicked() {
		s.commentStory = 0
		s.lyricsOpen = false
		s.page = name
		s.query = ""
		if s.remoteResults != nil && (name == youtube || name == "每日精选" || name == "钢琴" || name == "纯音乐") {
			s.requestRemote(name, false)
		}
	}
}
func (s *studio) prepareFrame(c *ui.Context) {
	s.poll()
	s.pollRemote()
	for _, loading := range s.remoteLoading {
		if loading {
			c.After(100 * time.Millisecond)
			break
		}
	}
	theme := darkGlass
	if s.Light {
		theme = lightGlass
		if !s.FrostedGlass {
			theme = lightSolid
		}
	} else if !s.FrostedGlass {
		theme = darkSolid
	}
	c.SetTheme(theme)
	if s.loading {
		c.After(16 * time.Millisecond)
	} else if (s.player != nil && s.player.Playing()) || s.lyricsStatus == "正在查找歌词…" {
		c.After(100 * time.Millisecond)
	} else {
		c.After(time.Minute)
	}
}
func (s *studio) view(c *ui.Context) {
	if s.page != youtube {
		s.closeVideo()
	}
	s.prepareFrame(c)
	s.daily(time.Now())
	videoOpen := s.page == youtube && s.videoSource != ""
	playerOpen := s.lyricsOpen || (videoOpen && !s.miniVideo)
	var target float32
	duration := 260 * time.Millisecond
	if playerOpen && !s.closingPlayer {
		target = 1
	}
	if s.closingPlayer {
		duration = 200 * time.Millisecond
	}
	progress := c.Root().AnimateWith("player-page", target, duration, ui.EaseInOut)
	if s.closingPlayer && progress == 0 {
		s.closingPlayer = false
		s.lyricsOpen = false
		if videoOpen {
			if s.videoFromRecent {
				s.page = recentPage
			}
			s.closeVideo()
		}
		playerOpen = false
	}
	if s.settingsOpen {
		s.settingsPage(c)
		return
	}
	if playerOpen {
		c.Root().Background(c.Theme().Background)
		ui.Box(c.Key("player-surface")).Fill().Translate(0, 36*(1-progress)).Opacity(progress).Children(func() {
			if videoOpen {
				s.youtubeListeningPage(c)
			} else {
				s.listeningPage(c)
			}
		})
		return
	}
	t := c.Theme()
	w, h := c.Size()
	rows := s.list()
	top, bottom := t.Background, s.color("#40303b", "#ece8f3")
	if s.Light && s.FrostedGlass && !c.Preferences().HighContrast {
		top, bottom = hex("#fff4eb"), hex("#eae2f3")
	}
	if s.FrostedGlass && c.Vibrancy() && !c.Preferences().HighContrast {
		c.Root().Background(ui.Transparent)
		top = top.Alpha(.83)
		bottom = bottom.Alpha(.68)
		if s.Light {
			top, bottom = top.Alpha(.58), bottom.Alpha(.48)
		}
	}
	bar := c.TitleBar()
	headerTop := float32(6)
	if bar.Left > 0 {
		// The inset macOS title bar includes empty space below its buttons.
		headerTop = min(bar.Height, 30)
	}
	extraHeight := headerTop - 16
	ui.Column(c).Fill().Gradient(top, bottom, 67).Padding(headerTop, 16, 16).Gap(12).Children(func() {
		ui.Row(c).Height(46).Padding(0, max(0, bar.Right-16), 0, 0).AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Row(c).Width(166).Height(36).Padding(0, 0, 0, 18).Gap(9).AlignItems(ui.Center).DragWindow().Children(func() {
				ui.Image(c, s.images["icon.png"]).Size(28, 28).Radius(8)
				ui.Icon(c, brandWordmark).Size(111, 19).TextColor(t.Text).Label("OmniMusic")
			})

			ui.Box(c).Grow(1).Height(42).DragWindow()
			toneLabel := locales.WarmTone()
			if s.OriginalSound {
				toneLabel = locales.OriginalTone()
			}
			if ui.Button(c.Key("tone"), toneLabel).Radius(18).TextColor(s.accent()).Tooltip(locales.ChangeTone()).Clicked() {
				s.OriginalSound = !s.OriginalSound
				s.bypass.Store(s.OriginalSound)
				s.save()
			}
			search := ui.TextInput(c.Key("search"), &s.query).Width(min(235, max(125, w-720))).Placeholder(locales.Search()).Label(locales.Search())
			if s.page != youtube && !search.Focused() && c.Shortcut(0, ui.KeySpace) {
				s.toggle()
			}
			if ui.Button(c.Key("settings"), "").Size(36, 36).Padding(0).Gap(0).Center().Radius(12).Label(locales.Settings()).Tooltip(locales.Settings()).Children(func() {
				icon(c, "settings", t.Text, 18)
			}).Clicked() {
				s.settingsOpen = true
			}
		})
		bodyH := h - 178 - extraHeight
		if s.page == youtube {
			bodyH = h - 90 - extraHeight
		}
		ui.Row(c).Height(bodyH).MinHeight(0).AlignItems(ui.Stretch).Gap(14).Children(func() {
			surfaceWash(c, ui.Column(c).Width(166), s.color("#47344718", "#ffffffd4"), s.color("#85657965", "#dfcde88c"), 165, 14).Padding(10, 10, 12).Gap(8).Children(func() {
				ui.Scroll(c.Key("navigation")).Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).Gap(5).Children(func() {
						txt(c, locales.Discover(), 12, t.TextMuted).Margin(3, 8, 7)
						s.nav(c, "每日精选", "sun")
						s.nav(c, youtube, "video")
						s.nav(c, "全部音乐", "disc")
						txt(c, locales.Library(), 12, t.TextMuted).Margin(22, 8, 7)
						s.nav(c, recentPage, "history")
						s.nav(c, "我的收藏", "heart")
						s.nav(c, "本地音乐", "folder")
						txt(c, locales.Categories(), 12, t.TextMuted).Margin(22, 8, 7)
						for i, g := range []string{"现代流行", "钢琴", "纯音乐"} {
							s.nav(c, g, []string{"wave", "piano", "strings"}[i])
						}
					})
				})
				if ui.Button(c, locales.ImportMusic()).Clicked() {
					s.importFiles()
				}
			})
			if s.page == youtube {
				s.youtubePage(c, rows, bodyH)
				return
			}
			ui.Column(c).Height(bodyH).Grow(1).MinWidth(0).Gap(12).Children(func() {
				if s.commentStory != 0 {
					s.commentsPage(c, bodyH)
				} else {
					s.hero(c, rows)
					s.tracks(c, rows, bodyH-208)
				}
			})
			if w >= 1000 {
				s.inspector(c)
			}
		})
		if s.page != youtube {
			s.transport(c, false)
		}
	})
}
func (s *studio) settingsPage(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	s.detailPage(c, "OmniMusic", func() { s.settingsOpen = false }, func() {
		ui.Column(c).Width(min(w-64, 820)).AlignSelf(ui.Center).Padding(min(h*.08, 72), 0).Gap(24).Children(func() {
			txt(c, locales.Settings(), 36, t.Text).Bold()
			glassPane(c, ui.Column(c).Radius(20)).Padding(24, 28).Gap(18).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					txt(c, locales.Language(), 15, t.Text).Grow(1)
					ui.MenuButton(c.Key("language"), i18n.Autonym(i18n.Locale()), func(m *ui.Menu) {
						if m.Item(locales.FollowSystem()).Chosen() {
							golet.App.SetLocale("")
						}
						for _, tag := range []string{"zh-Hans", "en"} {
							if m.Item(i18n.Autonym(tag)).Radio(i18n.Locale() == tag).Chosen() {
								golet.App.SetLocale(tag)
							}
						}
					})
				})
				ui.Box(c).Height(1).Background(t.Border)
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					txt(c, locales.Appearance(), 15, t.Text).Grow(1)
					selected := 0
					if !s.Light {
						selected = 1
					}
					ui.Segmented(c.Key("appearance"), &selected, locales.Light(), locales.Dark())
					if light := selected == 0; light != s.Light {
						s.Light = light
						s.applyAppearance()
						s.save()
					}
				})
				ui.Box(c).Height(1).Background(t.Border)
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					txt(c, locales.FrostedGlass(), 15, t.Text).Grow(1)
					if ui.Switch(c.Key("frosted-glass"), &s.FrostedGlass).Label(locales.ToggleGlass()).Changed() {
						if s.window != nil {
							s.window.SetVibrancy(s.windowMaterial())
						}
						s.save()
					}
				})
			})
		})
	})
}
func (s *studio) hero(c *ui.Context, rows []track) {
	ui.Box(c).Height(196).Radius(17).Clip().Children(func() {
		ui.Image(c, s.images["ribbon.jpg"]).Fill().Fit(ui.Cover)
		if s.Light {
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Gradient(hex("#f7d6d7f5"), hex("#efddf29e"), 105)
		}
		ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Padding(19, 23).AlignItems(ui.Start).Gap(5).Children(func() {
			txt(c, locales.DateLabel(time.Now()), 12, s.color("#e9dbea", "#564253")).Bold()
			title := topicName(s.page)
			txt(c, title, 46, s.color("#ffffff", "#302137")).Bold().SingleLine()
			txt(c, locales.TrackCount(len(rows)), 14, s.color("#e6deec", "#564253"))
			play := ui.PrimaryButton(c.Key("play-all"), "").Absolute().Right(26).Top(72).
				Size(52, 52).Radius(26).Padding(0).Center().Label(locales.PlayMix()).
				Tooltip(locales.PlayMix()).Disabled(len(rows) == 0)
			play.Children(func() { icon(c, "play", c.Theme().AccentText, 21) })
			if len(rows) > 0 {
				surfaceWash(c, play, s.color("#df947ca6", "#c87b93ac"), s.color("#64399970", "#7341a8a0"), 125, 26)
			}
			if play.Clicked() {
				s.playList(rows[0], rows)
			}
		})
	})
}
func (s *studio) refreshButton(c *ui.Context) bool {
	b := ui.Button(c.Key("refresh-mix"), "").Height(30).Padding(0, 10).Gap(5).Radius(9).Role(ui.RoleButton).Label(locales.Refresh()).Tooltip(locales.RefreshHint())
	accent := s.accent()
	b.Children(func() { icon(c, "refresh", accent, 14); txt(c, locales.Refresh(), 12, accent) })
	return b.Clicked()
}
func (s *studio) tracks(c *ui.Context, rows []track, height float32) {
	t := c.Theme()
	surfaceWash(c, glassPane(c, ui.Column(c).Height(height).MinHeight(0).Radius(16)), s.color("#69607342", "#ffffff18"), s.color("#342d2922", "#faf3f318"), 115, 16).Padding(14, 14, 5).Gap(7).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			txt(c, topicName(s.page), 20, t.Text).Bold()
			txt(c, locales.TrackCount(len(rows)), 12, t.TextMuted)
			ui.Box(c).Grow(1)
			if w, _ := c.Size(); w < 1000 && s.page == "每日精选" {
				if ui.Button(c, locales.HnComments()).Clicked() {
					s.showComments()
				}
			}
			if s.page == youtube || s.page == "每日精选" || s.page == "钢琴" || s.page == "纯音乐" {
				if s.refreshButton(c) {
					if s.page == youtube {
						s.videoBatch++
						s.recommendVideos()
					} else if s.page == "每日精选" {
						s.nextBatch()
					} else {
						s.genreBatch[s.page]++
						s.recommendGenre(s.page)
					}
					s.query = ""
					rows = s.list()
				}
			}
		})
		if len(rows) > 0 && s.remotePending[s.page] {
			status := locales.RemoteLoading()
			if !s.remoteLoading[s.page] {
				status = locales.RecommendationExhausted()
				if s.recommendationErrors[s.page] != "" {
					status = locales.RecommendationFailure(s.recommendationErrors[s.page])
				}
			}
			txt(c, status, 12, t.TextMuted).MaxLines(2)
		}
		if len(rows) == 0 {
			msg := locales.NoMatches()
			if s.query == "" && (s.page == youtube || s.page == "每日精选" || s.page == "钢琴" || s.page == "纯音乐") {
				msg = locales.RecommendationExhausted()
				if s.remoteLoading[s.page] {
					msg = locales.RemoteLoading()
				}
				if s.recommendationErrors[s.page] != "" {
					msg = locales.RecommendationFailure(s.recommendationErrors[s.page])
				}
			}
			if s.query == "" && s.page == "本地音乐" {
				msg = locales.ImportHint()
			} else if s.query == "" && s.page == "我的收藏" {
				msg = locales.LikeHint()
			} else if s.query == "" && s.page == recentPage {
				msg = locales.RecentHint()
			}
			txt(c, msg, 15, t.TextMuted).Margin(20)
			return
		}
		ui.Scroll(c.Key(s.page)).Height(height - 55).MinHeight(0).Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				for i, item := range rows {
					removed := false
					r := ui.Row(c.Key(item.Source)).Height(53).Radius(8).Padding(4, 8).AlignItems(ui.Center).Gap(10).Role(ui.RoleButton).Focusable().Label(item.Title)
					col := t.Text
					if item.Source == s.current.Source || (s.page == youtube && item.Source == s.videoSource) {
						col = s.accent()
						r.Background(s.color("#37303e", "#f3ebf0"))
					} else if r.Hovered() {
						r.Background(t.SurfaceHover)
					}
					r.Children(func() {
						txt(c, fmt.Sprintf("%02d", i+1), 11, t.TextMuted).Width(19)
						ui.Image(c, s.images[cover(item)]).Size(38, 38).Radius(6)
						ui.Column(c).Grow(1).MinWidth(0).Justify(ui.Center).Children(func() {
							txt(c, item.Title, 14, col).Bold().SingleLine().Ellipsis("…")
							v := item.Composer
							if v == "" {
								v = item.artistLabel()
							}
							txt(c, v, 12, t.TextMuted).SingleLine()
						})
						txt(c, item.Format, 10, s.accent())
						if item.Seconds > 0 {
							txt(c, clockText(item.Seconds), 12, t.TextMuted).Width(36).TextAlign(ui.End)
						}
						if s.page == "本地音乐" {
							removed = action(c, "remove", locales.RemoveLocal(item.Title), t.TextMuted, 30)
						}
					})
					if removed {
						s.removeLocal(item.Source)
						return
					}
					if r.Clicked() {
						if item.Format == youtube {
							s.playList(item, rows)
							continue
						}
						if item.Source != s.current.Source || (s.player == nil && !s.loading) {
							s.playList(item, rows)
						} else if s.player != nil && !s.player.Playing() {
							s.resumePlayback()
						}
					}
				}
			})
		})
	})
}
func (s *studio) inspector(c *ui.Context) {
	t := c.Theme()
	surfaceWash(c, glassPane(c, ui.Scroll(c).Width(235).Radius(16)), s.color("#3f384c18", "#ffffff22"), s.color("#9766554f", "#eacddc62"), 160, 16).Children(func() {
		ui.Column(c).Padding(16).Gap(10).Children(func() {
			txt(c, locales.RecordRoom(), 18, t.Text).Bold()
			if ui.Column(c.Key("inspect-track")).Gap(8).Focusable().Role(ui.RoleButton).Label(locales.OpenPlayer()).Children(func() {
				ui.Box(c).Height(171).Radius(12).Clip().Gradient(s.color("#645065", "#eee4dc"), s.color("#25243a", "#d2c6ce"), 28).Center().Children(func() {
					disc := ui.Image(c, s.images["record.png"]).Size(154, 154)
					if s.player != nil {
						disc.Rotate(float32(math.Mod(s.player.Position().Seconds()*12, 360)))
					}
				})
				txt(c, s.current.Title, 17, t.Text).Bold().MaxLines(2).Ellipsis("…")
				txt(c, s.current.artistLabel(), 12, t.TextMuted).MaxLines(2)
				s.lyricView(c)
			}).Clicked() {
				s.lyricsOpen = true
			}
			ui.Box(c).Height(1).Background(t.Border).MarginY(4)
			txt(c, locales.HnComments(), 15, t.Text).Bold()
			replies := s.comments[s.signal.ID]
			if len(replies) > 0 {
				txt(c, replies[0].Body, 14, t.Text).MaxLines(3).Ellipsis("…")
			} else {
				txt(c, locales.NoComments(), 13, t.TextMuted)
			}
			if ui.Button(c, locales.ReadComments()).Clicked() {
				s.showComments()
			}
			if s.current.Digest != "" || s.current.Remote {
				label, link := locales.Musopen(), "https://archive.org/details/MusopenCollectionAsFlac"
				if s.current.Release != "" {
					label = locales.License()
					link = s.current.Release
				}
				if ui.Button(c, label).Clicked() {
					_ = golet.Shell.OpenExternal(link)
				}
			}
		})
	})
}
func (s *studio) lyricView(c *ui.Context) {

	msg := locales.Instrumental()
	pos := 0.
	if s.player != nil {
		pos = s.player.Position().Seconds()
	}
	since := 1.
	if s.current.Release != "" {
		msg = lyricStatusName(s.lyricsStatus)
		if len(s.lines) == 0 && s.plainLyrics != "" {
			msg = locales.FullLyrics()
		}
	}
	if s.current.Digest == "" && !s.current.Remote {
		msg = locales.LocalLyrics()
	}
	for _, l := range s.lines {
		if l.at > pos {
			break
		}
		msg = l.text
		since = pos - l.at
	}
	txt(c, msg, 14, s.accent()).MaxLines(3).Opacity(float32(.55 + .45*min(1., since/.35)))
}

// The playback clock moves the ribbons; the decoded signal sets their height.
func drawMusicWave(p *ui.Painter, r ui.Rect, level float32, phase float64, color ui.Color) {
	var wave ui.Path
	wave.MoveTo(r.X, r.Y+r.H)
	for i := 0; i <= 60; i++ {
		x := float64(i) / 60
		crest := .5 + .32*math.Sin(x*11-phase*2.2) + .18*math.Sin(x*6+phase*1.3)
		wave.LineTo(r.X+float32(x)*r.W, r.Y+r.H-r.H*level*(.14+.52*float32(crest)))
	}
	wave.LineTo(r.X+r.W, r.Y+r.H).Close()
	p.FillPathGradient(&wave, ui.LinearGradient{From: color.Alpha(.18 + .12*level), To: color.Alpha(.035), Angle: 180})
}

func (s *studio) playbackBackground(c *ui.Context, height, radius, level float32, phase float64) ui.Element {
	bar := glassPane(c, ui.Row(c).Height(height).Radius(radius))
	if c.Preferences().HighContrast {
		return bar
	}
	if c.Preferences().ReduceMotion {
		level = 0
	}
	from, to := s.color("#73607c42", "#efe7f582"), s.color("#ae70609c", "#edbfa8aa")
	bar.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{From: from, To: to, Angle: 100}, radius)
		if level > .003 {
			p.Clip(r, radius, func() {
				drawMusicWave(p, r, level, phase, s.color("#d39adf", "#9755ae"))
				drawMusicWave(p, r, level*.72, phase*.74+2, s.color("#ffb89c", "#db8b73"))
			})
		}
	})
	return bar
}

func (s *studio) transport(c *ui.Context, expanded bool) {
	t := c.Theme()
	playing := s.player != nil && s.player.Playing()
	level, phase := float32(0), 0.
	if playing && s.meter != nil && !c.Preferences().ReduceMotion && !c.Preferences().HighContrast {
		level = min(1, 3*math.Float32frombits(s.meter.level.Load()))
		phase = s.player.Position().Seconds()
		c.After(time.Second / 30)
	}
	bar := s.playbackBackground(c, 76, 18, level, phase)
	bar.Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		if !expanded {
			if ui.Row(c.Key("track-details")).AlignItems(ui.Center).Gap(12).Focusable().Role(ui.RoleButton).Label(locales.OpenPlayer()).Children(func() {
				ui.Image(c, s.images[cover(s.current)]).Size(46, 46).Radius(8)
				ui.Column(c).Width(174).Justify(ui.Center).Children(func() {
					txt(c, s.current.Title, 14, t.Text).Bold().SingleLine().Ellipsis("…")
					sub := s.current.artistLabel()
					if s.message != nil {
						sub = s.message()
					}
					if s.loading && s.current.Size > 0 && s.progress != nil {
						sub = locales.LoadingProgress(s.current.Format, int(math.Round(float64(s.progress.bytes.Load())*100/float64(s.current.Size))))
					}
					w, _ := c.Size()
					if w < 1000 && len(s.lines) > 0 && s.message == nil {
						s.lyricView(c)
					} else {
						txt(c, sub, 12, t.TextMuted).SingleLine().Ellipsis("…").Tooltip(sub)
					}
				})
			}).Clicked() {
				s.lyricsOpen = true
			}
		}
		likeColor := t.TextMuted
		if s.Likes[s.current.Source] {
			likeColor = s.accent()
		}
		if action(c, "heart", locales.Like(), likeColor, 30) {
			s.like()
		}
		if action(c, "prev", locales.Previous(), t.Text, 32) {
			s.step(-1)
		}
		g := "play"
		label := locales.Play()
		if playing {
			g = "pause"
			label = locales.Pause()
		} else if s.loading {
			g = "stop"
			label = locales.CancelLoad()
		}
		b := ui.PrimaryButton(c.Key("toggle"), "").Size(45, 45).Padding(0).Center().Radius(23).Label(label).Tooltip(label)
		surfaceWash(c, b, s.color("#df947ca6", "#c87b93ac"), s.color("#64399970", "#7341a8a0"), 125, 23).Children(func() { icon(c, g, t.AccentText, 20) })
		if b.Clicked() {
			s.toggle()
		}
		if action(c, "next", locales.Next(), t.Text, 32) {
			s.step(1)
		}
		pos := 0.
		dur := s.current.Seconds
		if s.player != nil {
			pos = s.player.Position().Seconds()
			dur = s.player.Duration().Seconds()
		}
		txt(c, clockText(pos), 12, t.TextMuted)
		if ui.Slider(c.Key("seek"), &pos, 0, max(dur, 1)).Grow(1).Disabled(s.player == nil || dur <= 0).Label(locales.Seek()).Changed() && s.player != nil {
			s.player.Seek(time.Duration(pos * float64(time.Second)))
		}
		txt(c, clockText(dur), 12, t.TextMuted)
		if !expanded {

			if ui.Button(c.Key("lyrics"), locales.Lyrics()).Tooltip(locales.ShowLyrics()).Clicked() {
				s.lyricsOpen = true
			}
		}
		icon(c, "volume", t.TextMuted, 17)
		if ui.Slider(c.Key("volume"), &s.volume, 0, 1).Width(70).Label(locales.Volume()).Changed() && s.player != nil {
			s.player.SetVolume(s.volume)
		}
	})
}

func (s *studio) applyAppearance() {
	if s.window == nil {
		return
	}
	source := golet.ThemeDark
	if s.Light {
		source = golet.ThemeLight
	}
	golet.Theme.SetSource(source)
	s.window.SetWindowButtonsTheme(source)
}
