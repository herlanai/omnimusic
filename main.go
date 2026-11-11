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
		if err := s.initTray(); err != nil {
			log.Printf("system tray unavailable: %v", err)
		}
		w.OnClose(func(e *golet.CloseEvent) {
			if s.tray != nil && !s.quitting {
				e.PreventDefault()
				w.Hide()
				s.updateTray()
			}
		})
		w.OnClosed(func() {
			s.window = nil
			if s.tray == nil {
				s.stopIfClosed()
			}
		})
	})
	if e := golet.App.Run(); e != nil {
		log.Fatal(e)
	}
}
