package main

import (
	"github.com/HyperparameterSI/golet/backend"
	"github.com/HyperparameterSI/golet/backend/ui"
	"log"
)

func (s *studio) windowMaterial() golet.Vibrancy {
	if s.FrostedGlass {
		return golet.VibrancyGlass
	}
	return golet.VibrancyNone
}

func main() {
	s := newStudio()
	golet.App.SetName("OmniMusic")
	golet.App.WhenReady(func() {
		w := golet.NewWindow(golet.WindowOptions{Title: "OmniMusic", TitleBarStyle: golet.TitleBarHiddenInset, Vibrancy: s.windowMaterial(), Width: 1240, Height: 790, MinWidth: 820, MinHeight: 610, Content: ui.View(s.view)})
		s.window = w
		s.applyAppearance()
		w.OnClosed(func() {
			s.window = nil
			s.stopIfClosed()
		})
	})
	if e := golet.App.Run(); e != nil {
		log.Fatal(e)
	}
}
