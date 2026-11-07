package main

import "github.com/HyperparameterSI/golet/backend/ui"

var brandWordmark = ui.MustParseSVG(readAsset("wordmark.svg"))

var paths = map[string]string{
	"history":  "M3 11a9 9 0 1 1 2 7M3 5v6h6M12 7v5l4 2",
	"remove":   "M3 6h18M9 6V3h6v3M6 6l1 15h10l1-15M10 10v7m4-7v7",
	"mini":     "M3 4h18v16H3Z M12 11h7v6h-7Z",
	"close":    "M5 5l14 14M19 5L5 19",
	"collapse": "M4 9l8 8 8-8",
	"settings": "M9.5 3h5l.6 2.5 2 .9 2.3-.8 2.2 4-1.8 1.7v2.2l1.8 1.7-2.2 4-2.3-.8-2 .9-.6 2.5h-5l-.6-2.5-2-.9-2.3.8-2.2-4 1.8-1.7v-2.2L2.4 9.6l2.2-4 2.3.8 2-.9Z M15 12a3 3 0 1 1-6 0 3 3 0 1 1 6 0",
	"video":    "M3 5h18v14H3Z M10 9l5 3-5 3Z",
	"popout":   "M12 4H5a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2h13a2 2 0 0 0 2-2v-7 M14 3h7v7 M21 3l-10 10",
	"refresh":  "M4 10a8 8 0 0 1 13.7-3.8L20 8.5 M20 3.5v5h-5 M20 14a8 8 0 0 1-13.7 3.8L4 15.5 M4 20.5v-5h5",
	"sun":      "M12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2 M16 12a4 4 0 1 1-8 0 4 4 0 1 1 8 0",
	"disc":     "M21 12a9 9 0 1 1-18 0 9 9 0 1 1 18 0 M14 12a2 2 0 1 1-4 0 2 2 0 1 1 4 0",
	"theme":    "M20 14a8 8 0 0 1-10-11 9 9 0 1 0 10 11Z",
	"folder":   "M3 6h7l2 3h9v11H3Z",
	"heart":    "M12 20 4 12C-1 5 8 1 12 8c4-7 13-3 8 4Z",
	"piano":    "M3 4h18v16H3Z M8 4v9m4-9v16m4-16v9",
	"strings":  "M5 18 18 5m-9 7 3 3m2-5 3 3M4 15c-4 6 2 9 6 5l2-3-5-5Z",
	"wave":     "M3 9v6m4-10v14m5-17v20m5-17v14m4-10v6",
	"play":     "M7 4 20 12 7 20Z", "pause": "M8 5v14m8-14v14", "stop": "M6 6h12v12H6Z", "next": "M5 5l11 7-11 7Z M19 5v14", "prev": "M19 5 8 12l11 7Z M5 5v14",
	"volume": "M3 9h4l5-4v14l-5-4H3Z M16 8q5 4 0 8m3-11q8 7 0 14",
}
var glyphs = map[string]*ui.SVG{}

func icon(c *ui.Context, name string, col ui.Color, size float32) {
	g := glyphs[name]
	if g == nil {
		fill := "none"
		if name == "play" || name == "stop" {
			fill = "currentColor"
		}
		g = ui.MustParseSVG([]byte(`<svg viewBox="0 0 24 24"><path fill="` + fill + `" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" stroke-linecap="round" d="` + paths[name] + `"/></svg>`))
		glyphs[name] = g
	}
	ui.Icon(c, g).Size(size, size).TextColor(col)
}
func action(c *ui.Context, name, label string, col ui.Color, size float32) bool {
	e := ui.Button(c.Key(label), "").MinHeight(size).Size(size, size).Padding(0).Radius(size / 2).Center().Label(label).Tooltip(label)
	e.Children(func() { icon(c, name, col, 18) })
	return e.Clicked()
}
