package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/HyperparameterSI/golet/backend/i18n"
	"math"
	"sort"
	"strings"
	"time"
)

type topicEvidence struct {
	strength          float64
	stories, mentions int
	example           signal
	best              float64
}

func summarize(rows []signal) map[string]topicEvidence {
	result := make(map[string]topicEvidence)
	for _, row := range rows {
		// Count each discussion once per topic. Logarithms temper viral outliers.
		support := math.Log1p(float64(row.Mentions)) * (1 + math.Log1p(float64(row.Score)) + .5*math.Log1p(float64(row.Comments)))
		e := result[row.Group]
		e.strength += support
		e.stories++
		e.mentions += row.Mentions
		if support > e.best {
			e.best = support
			e.example = row
		}
		result[row.Group] = e
	}
	return result
}

// Exponential-race sampling gives larger weights a better chance, without replacement.
func sampleRank(seed, key string, weight float64) float64 {
	digest := sha256.Sum256([]byte(seed + "|" + key))
	unit := (1 + float64(binary.BigEndian.Uint32(digest[:4]))) / 4294967297
	return -math.Log(unit) / weight
}

func (s *studio) daily(now time.Time) {
	date := now.Format("2006-01-02")
	lang, _, _ := strings.Cut(i18n.Locale(), "-")
	if date == s.date && lang == s.mixLanguage {
		return
	}
	s.mixLanguage = lang
	s.date = date
	s.videoDay = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
	s.videoBatch = 0
	s.recommendVideos()
	s.batch = 0
	s.mix = nil
	s.genreMix = map[string][]track{}
	s.genreBatch = map[string]int{}
	s.recommend()
	s.recommendGenre("钢琴")
	s.recommendGenre("纯音乐")
}
func (s *studio) nextBatch() { s.batch++; s.recommend() }
func (s *studio) recommend() {
	seed := fmt.Sprintf("%s/%d", s.date, s.batch)
	// Topic prior is smoothed so a small corpus can still surface quieter genres.
	best := math.Inf(1)
	var focus string
	for group, e := range s.evidence {
		if group != "独立流行" && group != "电子流行" {
			continue
		}
		rank := sampleRank(seed, group, 1+math.Log1p(e.strength))
		if rank < best || (rank == best && group < focus) {
			best = rank
			focus = group
		}
	}
	s.signal = s.evidence[focus].example
	previous := make(map[string]bool, len(s.mix))
	for _, t := range s.mix {
		previous[t.Source] = true
	}
	counts := map[string]int{}
	for _, t := range s.catalog {
		counts[t.Group]++
	}
	type candidate struct {
		track track
		rank  float64
		seen  bool
	}
	candidates := make([]candidate, 0, len(s.catalog))
	for _, t := range s.catalog {
		if t.Release == "" || t.Format == youtube {
			continue
		}
		weight := (1 + math.Log1p(s.evidence[t.Group].strength)) / float64(counts[t.Group])
		if t.Group == focus {
			weight *= 2.5
		}
		candidates = append(candidates, candidate{t, sampleRank(seed, t.Source, weight), previous[t.Source]})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.seen != b.seen {
			return !a.seen
		}
		return a.rank < b.rank
	})
	s.mix = nil
	selected := map[string]bool{}
	// Lead with up to four newly suggested recordings in the effective UI language.
	// Reserve half the small language pool for the next batch.
	matches := 0
	for _, p := range candidates {
		if p.track.Language == s.mixLanguage {
			matches++
		}
	}
	lead := min(4, (matches+1)/2)
	for _, p := range candidates {
		if len(s.mix) >= lead {
			break
		}
		if !p.seen && p.track.Language == s.mixLanguage {
			s.mix = append(s.mix, p.track)
			selected[p.track.Source] = true
		}
	}
	// Fill with other languages first, keeping unseen tracks ahead of repeats.
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.seen != b.seen {
			return !a.seen
		}
		return a.track.Language != s.mixLanguage && b.track.Language == s.mixLanguage
	})
	for _, p := range candidates {
		if len(s.mix) >= 12 {
			break
		}
		if !selected[p.track.Source] {
			s.mix = append(s.mix, p.track)
		}
	}
}

func (s *studio) recommendGenre(genre string) {
	seed := fmt.Sprintf("%s/%s/%d", s.date, genre, s.genreBatch[genre])
	seen := map[string]bool{}
	for _, t := range s.genreMix[genre] {
		seen[t.Source] = true
	}
	var pool []track
	for _, t := range s.catalog {
		if t.Format != youtube && t.Release == "" && ((genre == "钢琴") == (t.Group == "钢琴")) {
			pool = append(pool, t)
		}
	}
	sort.SliceStable(pool, func(i, j int) bool {
		a, b := pool[i], pool[j]
		if seen[a.Source] != seen[b.Source] {
			return !seen[a.Source]
		}
		return sampleRank(seed, a.Source, 1+math.Log1p(s.evidence[a.Group].strength)) < sampleRank(seed, b.Source, 1+math.Log1p(s.evidence[b.Group].strength))
	})
	s.genreMix[genre] = pool[:min(12, len(pool))]
}

// Alternate two disjoint pools by civil date and batch so a fresh launch
// produces the same daily picks without storing the previous day's list.
func (s *studio) recommendVideos() {
	seed := fmt.Sprintf("youtube/%s/%d", s.date, s.videoBatch)
	pool := make([]track, 0, len(videoCatalog)/2)
	ranks := make(map[string]float64, len(videoCatalog)/2)
	for i, song := range videoCatalog {
		if (int64(i)+s.videoDay+int64(s.videoBatch))%2 != 0 {
			continue
		}
		weight := 1 + math.Log1p(s.evidence[song.Group].strength)
		if song.Language == s.mixLanguage {
			weight *= 2
		}
		ranks[song.Source] = sampleRank(seed, song.Source, weight)
		pool = append(pool, song)
	}
	sort.SliceStable(pool, func(i, j int) bool { return ranks[pool[i].Source] < ranks[pool[j].Source] })
	s.videoMix = pool[:min(12, len(pool))]
}
