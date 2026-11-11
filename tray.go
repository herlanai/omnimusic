package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"time"

	golet "github.com/HyperparameterSI/golet/backend"
	"omnimusic/locales"
)

// A monochrome waveform stays legible on both light and dark system bars.
func statusIcon() []byte {
	im := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for i, height := range []int{10, 20, 28, 20, 12} {
		for x := 2 + i*6; x < 6+i*6; x++ {
			for y := (32 - height) / 2; y < (32+height)/2; y++ {
				im.Set(x, y, color.NRGBA{A: 255})
			}
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

func compactSong(title string) string {
	r := []rune(strings.ReplaceAll(title, "\n", " "))
	if len(r) > 24 {
		return string(r[:24]) + "…"
	}
	return string(r)
}

func (s *studio) showWindow() {
	if s.window != nil {
		s.window.Show()
		s.window.Focus()
	}
}

func (s *studio) initTray() error {
	t, err := golet.NewTray(golet.TrayOptions{ID: "app.omnimusic.desktop.player", Icon: statusIcon(), IconIsTemplate: true, ToolTip: "OmniMusic"})
	if err != nil {
		return err
	}
	s.tray = t
	s.updateTray()
	// UI drawing stops while hidden. Keep audio completion, loading results and
	// automatic next-track handling on the main thread, independently of frames.
	done := make(chan struct{})
	golet.App.OnBeforeQuit(func(*golet.QuitEvent) { s.quitting = true })
	golet.App.OnQuit(func() { close(done); s.stopIfClosed() })
	golet.App.OnActivate(func(visible bool) {
		if !visible && !s.quitting {
			s.showWindow()
		}
	})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				golet.App.WhenReady(func() {
					if s.quitting {
						return
					}
					s.poll()
					s.pollRemote()
					s.updateTray()
				})
			}
		}
	}()
	return nil
}

func (s *studio) updateTray() {
	if s.tray == nil {
		return
	}
	title, artist := s.current.Title, s.current.artistLabel()
	video := s.videoSource != "" && s.videoView != nil
	playing := s.player != nil && s.player.Playing()
	if video {
		title, artist = s.videoLabels()
		playing = !s.videoAudibleSince.IsZero()
	}
	info := locales.TrayIdle()
	if video || s.player != nil || s.loading {
		info = title
		if artist != "" {
			info += " — " + artist
		}
	}
	bar := ""
	if playing {
		bar = "♫ " + compactSong(title)
	}
	toggle := locales.TrayPlay()
	if playing {
		toggle = locales.TrayPause()
	}
	// Rebuild only on changed metadata, playback state, or language.
	key := strings.Join([]string{info, bar, toggle, locales.TrayOpen(), s.current.Source, s.videoSource}, "\x00")
	if key == s.trayState {
		return
	}
	s.trayState = key
	s.tray.SetTitle(bar)
	s.tray.SetToolTip("OmniMusic · " + info)
	items := []*golet.MenuItem{
		{Label: info, Disabled: true}, golet.Separator(),
		{Label: locales.TrayOpen(), Click: func(*golet.MenuItem, *golet.Window) { s.showWindow() }},
	}
	if video {
		items = append(items, &golet.MenuItem{Label: locales.VideoPlayer(), Click: func(*golet.MenuItem, *golet.Window) { s.miniVideo = false; s.showWindow(); s.window.Invalidate() }})
	} else {
		items = append(items,
			&golet.MenuItem{Label: toggle, Disabled: s.current.Source == "", Click: func(*golet.MenuItem, *golet.Window) { s.toggle(); s.updateTray() }},
			&golet.MenuItem{Label: locales.TrayPrevious(), Disabled: len(s.queue) == 0, Click: func(*golet.MenuItem, *golet.Window) { s.step(-1); s.updateTray() }},
			&golet.MenuItem{Label: locales.TrayNext(), Disabled: len(s.queue) == 0, Click: func(*golet.MenuItem, *golet.Window) { s.step(1); s.updateTray() }},
		)
	}
	items = append(items, golet.Separator(), &golet.MenuItem{Label: locales.TrayQuit(), Click: func(*golet.MenuItem, *golet.Window) { golet.App.Quit() }})
	s.tray.SetMenu(golet.NewMenu(items))
}
