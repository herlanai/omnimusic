package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRemoteArchiveMetadata(t *testing.T) {
	var item archiveItem
	err := json.Unmarshal([]byte(`{"metadata":{"title":"Album","creator":["Artist"],"language":"Chinese","licenseurl":"https://creativecommons.org/licenses/by/4.0/"},"files":[{"name":"song.flac","title":"Song","sha1":"0123456789012345678901234567890123456789","size":"12345","length":"3:12"},{"name":"cover.jpg"}]}`), &item)
	if err != nil {
		t.Fatal(err)
	}
	rows := archiveTracks(item, "album-id", "每日精选")
	if len(rows) != 1 || !rows[0].Remote || rows[0].Language != "zh" || rows[0].Seconds != 192 || rows[0].Format != "FLAC" {
		t.Fatalf("bad metadata: %+v", rows)
	}
	item.Metadata.Licenseurl = ""
	if len(archiveTracks(item, "album-id", "每日精选")) != 0 {
		t.Fatal("unlicensed item entered recommendations")
	}
}

func TestRemoteNeverUsesBundledAudio(t *testing.T) {
	hit := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		http.Error(w, "offline", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	_, err := openTrack(context.Background(), track{Remote: true, Digest: badSignDigest, Source: server.URL, Format: "MP3", Size: 1}, new(transferProgress))
	if err == nil || !hit {
		t.Fatal("remote request fell back to bundled music")
	}
}

func TestLiveRemoteCatalog(t *testing.T) {
	if os.Getenv("OMNI_LIVE_REMOTE") != "1" {
		t.Skip("live provider verification is opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 55*time.Second)
	defer cancel()
	rows, err := fetchArchive(ctx, "每日精选", "zh", 1)
	if err != nil || len(rows) == 0 {
		t.Fatalf("provider: %v (%d tracks)", err, len(rows))
	}
	t.Logf("received %d tracks; first=%s / %s", len(rows), rows[0].Title, rows[0].Source)
	file, err := openTrack(ctx, rows[0], new(transferProgress))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	decoder, err := decodeTrack(file, rows[0])
	if err != nil {
		t.Fatal(err)
	}
	buffer := make([]float32, 4096)
	n, err := decoder.Read(buffer)
	if n == 0 {
		t.Fatalf("no decoded samples: %v", err)
	}
	t.Logf("decoded %d samples at %d Hz", n, decoder.SampleRate())
}

func TestLiveYouTubeCatalog(t *testing.T) {
	if os.Getenv("OMNI_LIVE_REMOTE") != "1" {
		t.Skip("live provider verification is opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	rows, err := fetchYouTube(ctx, "zh", 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("HN returned %d verified YouTube entries", len(rows))
}

type catalogRoundTrip func(*http.Request) (*http.Response, error)

func (f catalogRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestProductionRecommendationsUseRemoteCatalog(t *testing.T) {
	old := catalogHTTP
	catalogHTTP = &http.Client{Transport: catalogRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := "{}"
		switch r.URL.Path {
		case "/advancedsearch.php":
			body = `{"response":{"docs":[{"identifier":"remote-album"}]}}`
		case "/metadata/remote-album":
			body = `{"metadata":{"title":"Remote album","creator":"Remote artist","language":"English","licenseurl":"https://creativecommons.org/licenses/by/4.0/"},"files":[{"name":"Remote song.mp3","title":"Remote song","sha1":"0123456789012345678901234567890123456789","size":"12345","length":"180"}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	defer func() { catalogHTTP = old }()
	s := newStudioWithRemote(filepath.Join(t.TempDir(), "recommendations.sqlite"), true)
	defer s.stopIfClosed()
	if len(s.catalog) != 0 || len(s.mix) != 0 || len(s.videoMix) != 0 {
		t.Fatal("embedded candidates exposed before remote fetch")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s.pollRemote()
		loading := false
		for _, pending := range s.remoteLoading {
			loading = loading || pending
		}
		if !loading {
			break
		}
		time.Sleep(time.Millisecond)
	}
	for _, pending := range s.remoteLoading {
		if pending {
			t.Fatal("remote requests did not finish")
		}
	}
	if len(s.catalog) != 1 || !s.catalog[0].Remote || s.catalog[0].Title != "Remote song" {
		t.Fatalf("wrong remote catalog: %v", s.catalog)
	}
	for _, song := range s.mix {
		if !song.Remote {
			t.Fatal("bundled recommendation")
		}
	}
}

func TestLiveAudiusCatalog(t *testing.T) {
	if os.Getenv("OMNI_LIVE_REMOTE") != "1" {
		t.Skip("live provider verification is opt-in")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	rows, err := fetchAudius(ctx, "每日精选", "zh", 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Audius: %d public tracks in %s", len(rows), time.Since(start))
	start = time.Now()
	r, err := openTrack(ctx, rows[0], new(transferProgress))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	d, err := decodeTrack(r, rows[0])
	if err != nil {
		t.Fatal(err)
	}
	n, err := d.Read(make([]float32, 4096))
	if n == 0 {
		t.Fatalf("no decoded samples: %v", err)
	}
	t.Logf("First decoded audio in %s (%d Hz)", time.Since(start), d.SampleRate())
}

func TestRefreshUsesInventoryWhileNetworkBusy(t *testing.T) {
	s := newTestStudio(t)
	s.remoteResults = make(chan remoteResult, 8)
	s.remotePools = map[string][]track{}
	s.remotePending = map[string]bool{}
	s.remoteWorkers = map[string]int{}
	s.remoteLoading = map[string]bool{"每日精选": true}
	s.mix = nil
	for i := 0; i < 72; i++ {
		s.remotePools["每日精选"] = append(s.remotePools["每日精选"], track{Title: fmt.Sprintf("remote %d", i),
			Artist: "Artist", Remote: true, StreamOnly: true, Format: "MP3", Source: fmt.Sprintf("https://api.audius.co/v1/tracks/%d/stream", i)})
	}
	if err := s.storeRemoteInventory("每日精选", s.remotePools["每日精选"]); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	start := time.Now()
	for range 6 {
		s.requestRemote("每日精选", true)
		if len(s.mix) != 12 || s.remotePending["每日精选"] {
			t.Fatal("busy network blocked cached refresh")
		}
		for _, song := range s.mix {
			if seen[song.Source] {
				t.Fatal("repeat within cooldown")
			}
			seen[song.Source] = true
		}
	}
	t.Logf("Six cached batches: %s", time.Since(start))
	last := s.mix[0]
	s.requestRemote("每日精选", true)
	if s.mix[0] != last || !s.remotePending["每日精选"] {
		t.Fatal("empty refresh lost existing list")
	}
	reopened := &studio{recommendationPath: s.recommendationPath, remotePools: map[string][]track{}}
	reopened.restoreRemoteInventory()
	if len(reopened.remotePools["每日精选"]) != 72 {
		t.Fatal("unseen inventory was not persisted")
	}
}

func TestAudiusExcludesGatedTracks(t *testing.T) {
	old := catalogHTTP
	defer func() { catalogHTTP = old }()
	catalogHTTP = &http.Client{Transport: catalogRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"public","title":"Public","duration":180,"is_available":true,"is_streamable":true},
		{"id":"gated","duration":180,"is_available":true,"is_streamable":true,"is_stream_gated":true},
		{"id":"blocked","duration":180,"is_available":false,"is_streamable":true}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}
	rows, err := fetchAudius(context.Background(), "每日精选", "en", 1)
	if err != nil || len(rows) != 1 || rows[0].Title != "Public" || !rows[0].StreamOnly {
		t.Fatalf("bad public filtering: %v %v", rows, err)
	}
}
