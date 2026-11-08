// Generates the Retina Finder background for the drag-to-install disk image.
package main

import (
	"image/png"
	"os"

	"github.com/HyperparameterSI/golet/backend/ui"
)

func main() {
	logoData, err := os.ReadFile("assets/icon.png")
	must(err)
	logo, err := ui.DecodeBitmap(logoData)
	must(err)
	wordmarkData, err := os.ReadFile("assets/wordmark.svg")
	must(err)
	wordmark := ui.MustParseSVG(wordmarkData)
	arrow := ui.MustParseSVG([]byte(`<svg viewBox="0 0 64 32"><path d="M4 16h52M44 5l12 11-12 11" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`))
	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Gradient(ui.Hex("#fff7f2"), ui.Hex("#ebe4f6"), 125).Children(func() {
			ui.Row(c).Absolute().Left(40).Top(32).Gap(13).AlignItems(ui.Center).Children(func() {
				ui.Image(c, logo).Size(42, 42).Radius(12)
				ui.Icon(c, wordmark).Size(180, 30).TextColor(ui.Hex("#35253f"))
			})
			ui.Text(c, "让音乐，与你相遇").Absolute().Left(40).Top(99).
				FontSize(26).Bold().TextColor(ui.Hex("#35253f"))
			ui.Text(c, "Your music. A little closer.").Absolute().Left(40).Top(139).
				FontSize(14).TextColor(ui.Hex("#786780"))
			ui.Box(c).Absolute().Left(45).Right(45).Top(197).Height(146).Radius(28).
				Background(ui.Hex("#ffffff68"))
			ui.Icon(c, arrow).Absolute().Left(328).Top(249).Size(64, 32).TextColor(ui.Hex("#8b63a9"))
			ui.Text(c, "将 OmniMusic 拖入 Applications，即可安装").Absolute().
				Left(0).Right(0).Top(373).TextAlign(ui.Center).FontSize(17).TextColor(ui.Hex("#493552"))
			ui.Text(c, "Drag OmniMusic into Applications to install").Absolute().
				Left(0).Right(0).Top(405).TextAlign(ui.Center).FontSize(13).TextColor(ui.Hex("#786780"))
		})
	}, 720, 460, 2)
	must(os.MkdirAll("build/installer", 0755))
	f, err := os.Create("build/installer/background@2x.png")
	must(err)
	must(png.Encode(f, img))
	must(f.Close())
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
