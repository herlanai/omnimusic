package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newTestStudio(t *testing.T) *studio {
	t.Helper()
	return newStudioWithCache(filepath.Join(t.TempDir(), "recommendations.sqlite"))
}

func TestRecommendationCooldownAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recommendations.sqlite")
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	pool := []track{
		{Title: "中文", Artist: "歌手", Language: "zh", Source: "https://archive.org/one"},
		{Title: "English", Artist: "Singer", Language: "en", Source: "https://archive.org/two"},
	}
	fetch := func(at time.Time, fresh bool) []track {
		t.Helper()
		rows, err := cachedRecommendations(path, at.Format("2006-01-02"), "daily", "zh", at, fresh, pool, nil)
		if err != nil {
			t.Fatal(err)
		}
		return rows
	}
	rows := fetch(now, false)
	if len(rows) != 2 || rows[0].Language != "zh" {
		t.Fatalf("language ordering: %v", rows)
	}
	if restored := fetch(now, false); len(restored) != 2 || restored[0] != rows[0] {
		t.Fatal("daily batch lost on reopen")
	}
	if rows := fetch(now, true); len(rows) != 0 {
		t.Fatal("refresh repeated recent songs")
	}
	if rows := fetch(now.Add(7*24*time.Hour-time.Second), false); len(rows) != 0 {
		t.Fatal("cooldown ended early")
	}
	if rows := fetch(now.Add(7*24*time.Hour), true); len(rows) != 2 {
		t.Fatalf("cooldown did not expire: %v", rows)
	}
	db, err := recommendationDB(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stored, err := db.Query(context.Background(), "SELECT id,title,url,provider_id FROM tracks")
	if err != nil || len(stored.Values) != 2 {
		t.Fatalf("metadata not cached: %v %v", stored, err)
	}
}

func TestRecommendationCrossSourceDedup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recommendations.sqlite")
	now := time.Now()
	song := track{Title: "One Song", Artist: "One Artist", Language: "en", Source: "https://archive.org/one"}
	pool := []track{song, song}
	pool[1].Source = "https://another.example/one"
	rows, err := cachedRecommendations(path, "day", "daily", "en", now, true, pool, nil)
	if err != nil || len(rows) != 1 {
		t.Fatalf("duplicate sources: %v %v", rows, err)
	}
	song.Source = "https://www.youtube.com/watch?v=123456789ab"
	song.Format = youtube
	rows, err = cachedRecommendations(path, "day", youtube, "en", now, true, []track{song}, nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-source cooldown: %v %v", rows, err)
	}
}

func TestConcurrentRecommendationReservations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recommendations.sqlite")
	db, err := recommendationDB(path)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	var pool []track
	for i := 0; i < 24; i++ {
		pool = append(pool, track{Title: fmt.Sprintf("Song %d", i), Artist: "Artist", Language: "en", Source: fmt.Sprintf("https://archive.org/%d", i)})
	}
	type result struct {
		rows []track
		err  error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			rows, err := cachedRecommendations(path, "day", "daily", "en", time.Now(), true, pool, nil)
			results <- result{rows, err}
		}()
	}
	seen := map[string]bool{}
	for range 2 {
		got := <-results
		if got.err != nil || len(got.rows) != 12 {
			t.Fatalf("reservation failed: %v %v", got.rows, got.err)
		}
		for _, song := range got.rows {
			id := recordingID(song)
			if seen[id] {
				t.Fatal("concurrent batches repeated a song")
			}
			seen[id] = true
		}
	}
}

func TestProviderIdentitySurvivesMetadataChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recommendations.sqlite")
	now := time.Now()
	song := track{Title: "Song", Artist: "Artist", Source: "https://archive.org/one", Digest: "same-file"}
	if _, err := cachedRecommendations(path, "day", "daily", "en", now, true, []track{song}, nil); err != nil {
		t.Fatal(err)
	}
	song.Title = "01 - Artist - Song"
	rows, err := cachedRecommendations(path, "day", "remote-v1/daily", "en", now, true, []track{song}, nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("renamed remote metadata bypassed cooldown: %v %v", rows, err)
	}
}
