package main

import (
	"image"
	"testing"

	"github.com/HyperparameterSI/golet/backend/ui"
	"omnimusic/locales"
)

func TestSettingsGlyphCentered(t *testing.T) {
	s := newStudio()
	for _, light := range []bool{false, true} {
		s.Light = light
		for _, width := range []int{820, 1240} {
			v := ui.NewTester(s.view, width, 610)
			v.SetTitleBar(ui.TitleBar{Height: 32, Right: 132})
			v.Frame()
			r, ok := v.Find(locales.Settings())
			if !ok || r.X+r.W > float32(width)-132 {
				t.Fatalf("settings overlaps window controls: %+v", r)
			}
			img := v.Image()
			var ink image.Rectangle
			for y := int(r.Y) + 4; y < int(r.Y+r.H)-4; y++ {
				for x := int(r.X) + 4; x < int(r.X+r.W)-4; x++ {
					px := img.RGBAAt(x, y)
					if (light && px.R < 100) || (!light && px.R > 180) {
						ink = ink.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			dx := float32(ink.Min.X+ink.Max.X)/2 - (r.X + r.W/2)
			dy := float32(ink.Min.Y+ink.Max.Y)/2 - (r.Y + r.H/2)
			if ink.Dx() < 12 || ink.Dy() < 12 || dx < -1 || dx > 1 || dy < -1 || dy > 1 {
				t.Fatalf("light=%v width=%d glyph=%v button=%+v", light, width, ink, r)
			}
		}
	}
}

func TestGlassSettingPersists(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("AppData", dir)
	s := newStudio()
	if !s.FrostedGlass {
		t.Fatal("glass should default to enabled")
	}
	s.settingsOpen = true
	v := ui.NewTester(s.view, 820, 610)
	v.Frame()
	if err := v.Click(locales.ToggleGlass()); err != nil {
		t.Fatal(err)
	}
	if s.FrostedGlass || newStudio().FrostedGlass {
		t.Fatal("glass switch did not persist")
	}
}

func TestLightGlassOnContrastingBackdrops(t *testing.T) {
	sample := func(background ui.Color) uint8 {
		view := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(lightGlass)
			ui.Box(c).Fill().Background(background).Center().Children(func() {
				glassPane(c, ui.Box(c).Size(80, 80).Radius(12))
			})
		}, 100, 100)
		view.Frame()
		return view.Image().RGBAAt(50, 50).R
	}
	dark, bright := sample(ui.RGB(0, 0, 0)), sample(ui.RGB(255, 255, 255))
	if dark < 240 || dark >= bright {
		t.Fatalf("expected near-white translucent glass: black backdrop %d, white backdrop %d", dark, bright)
	}
}
