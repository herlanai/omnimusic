package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/HyperparameterSI/golet/backend/i18n"
	"math"
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
	s.recommendationTime = now
	s.mixLanguage = lang
	s.date = date
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
	if s.remoteResults != nil {
		s.requestRemote("每日精选", s.batch > 0)
		return
	}

	var pool []track
	for _, t := range s.catalog {
		if t.Release != "" && t.Format != youtube {
			pool = append(pool, t)
		}
	}
	s.mix = s.cachedMix("每日精选", s.batch > 0, pool)
}

func (s *studio) recommendGenre(genre string) {
	if s.remoteResults != nil {
		s.requestRemote(genre, s.genreBatch[genre] > 0)
		return
	}
	var pool []track
	for _, t := range s.catalog {
		if t.Format != youtube && t.Release == "" && ((genre == "钢琴") == (t.Group == "钢琴")) {
			pool = append(pool, t)
		}
	}
	s.genreMix[genre] = s.cachedMix(genre, s.genreBatch[genre] > 0, pool)
}

func (s *studio) recommendVideos() {
	if s.remoteResults != nil {
		s.requestRemote(youtube, s.videoBatch > 0)
		return
	}
	s.videoMix = s.cachedMix(youtube, s.videoBatch > 0, videoCatalog)
}

func (s *studio) cachedMix(channel string, refresh bool, pool []track) []track {
	now := s.recommendationTime
	if refresh {
		now = time.Now()
	}
	cacheChannel := channel
	if s.remoteResults != nil {
		cacheChannel = "remote-v1/" + channel
	}
	rows, err := cachedRecommendations(s.recommendationPath, s.date, cacheChannel, s.mixLanguage, now, refresh, pool, s.evidence)
	if err != nil {
		s.recommendationErrors[channel] = err.Error()
		return nil
	}
	delete(s.recommendationErrors, channel)
	return rows
}
