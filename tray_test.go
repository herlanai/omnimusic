package main

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestStatusIconAndTitle(t *testing.T) {
	icon, err := png.Decode(bytes.NewReader(statusIcon()))
	if err != nil || icon.Bounds().Dx() != 32 || icon.Bounds().Dy() != 32 {
		t.Fatalf("invalid status icon: %v", err)
	}
	_, _, _, corner := icon.At(0, 0).RGBA()
	_, _, _, bar := icon.At(15, 15).RGBA()
	if corner != 0 || bar == 0 {
		t.Fatal("icon needs transparent background and visible waveform")
	}
	title := compactSong(strings.Repeat("中文", 30))
	if !utf8.ValidString(title) || len([]rune(title)) != 25 || !strings.HasSuffix(title, "…") {
		t.Fatal("title truncation damaged unicode")
	}
	if compactSong("A\nB") != "A B" {
		t.Fatal("status title contains newline")
	}
}
