package main

import (
	"omnimusic/locales"

	"github.com/HyperparameterSI/golet/backend/ui"
)

type hnComment struct {
	ID           int
	Author, Body string
}

func (s *studio) showComments() {
	s.commentStory = s.signal.ID
	s.commentLimit = 16
	s.lyricsOpen = false
}
func (s *studio) commentsPage(c *ui.Context, height float32) {
	t := c.Theme()
	replies := s.comments[s.commentStory]
	glassPane(c, ui.Column(c).Height(height).Radius(16)).Padding(18).Gap(14).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			txt(c, locales.HnComments(), 20, t.Text).Bold()
			ui.Box(c).Grow(1)
			if ui.Button(c, locales.Back()).Clicked() {
				s.commentStory = 0
			}
		})
		if len(replies) == 0 {
			txt(c, locales.NoComments(), 14, t.TextMuted)
			return
		}
		ui.Scroll(c.Key(s.commentStory)).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Gap(10).Children(func() {
				for _, reply := range replies[:min(len(replies), s.commentLimit)] {
					txt(c, reply.Author, 12, s.accent()).Bold()
					txt(c, reply.Body, 15, t.Text)
					ui.Box(c).Height(1).Background(t.Border)
				}
				if s.commentLimit < len(replies) && ui.Button(c, locales.MoreComments()).Clicked() {
					s.commentLimit += 16
				}
			})
		})
	})
}
func (s *studio) stopIfClosed() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.player != nil {
		s.player.Close()
	}
}

func (s *studio) listeningPage(c *ui.Context) {
	t := c.Theme()
	w, h := c.Size()
	margin := min(float32(86), w*.055)
	side := min(w*.32, min(h-210, 480))
	if c.Shortcut(0, ui.KeySpace) {
		s.toggle()
	}
	s.detailPage(c, locales.NowPlaying(), func() { s.closingPlayer = true; c.AnimationFrame() }, func() {
		ui.Row(c).Grow(1).MinHeight(0).Padding(12, margin).AlignItems(ui.Center).Gap(margin).Children(func() {
			ui.Box(c).Size(side, side).Radius(16).Shadow(0, 14, 36, 0, hex("#170f2438")).Children(func() {
				ui.Image(c, s.images[cover(s.current)]).Fill().Fit(ui.Cover).Radius(16)
			})
			ui.Column(c).Grow(1).MinWidth(0).AlignSelf(ui.Stretch).Justify(ui.Center).Padding(16, 0).Gap(12).Children(func() {
				txt(c, s.current.Title, min(float32(40), w*.031), t.Text).Bold().MaxLines(2).Ellipsis("…")
				txt(c, s.current.artistLabel(), 17, t.TextMuted)
				ui.Box(c).Height(1).Background(t.TextMuted.Alpha(.23)).MarginY(6)
				s.lyricsBody(c)
				if s.message != nil {
					txt(c, s.message(), 13, t.TextMuted)
				}
			})
		})
		s.transport(c, true)
	})
}

func (s *studio) detailPage(c *ui.Context, title string, back func(), content func()) {
	bar := c.TitleBar()
	t := c.Theme()
	if c.Shortcut(0, ui.KeyEscape) {
		back()
	}
	ui.Column(c.Key("listening")).Fill().Gradient(s.color("#483b42", "#faf8f5"), s.color("#191820", "#efe9ed"), 65).Padding(12, 16, 16).Gap(12).Children(func() {
		ui.Row(c).Height(40).Padding(0, max(0, bar.Right-16), 0, max(0, bar.Left-16)).AlignItems(ui.Center).Gap(10).Children(func() {
			if action(c, "collapse", locales.Back(), t.Text, 34) {
				back()
				return
			}
			txt(c, title, 14, t.TextMuted).DragWindow()
			ui.Box(c).Grow(1).Height(40).DragWindow()
		})
		content()
	})
}
