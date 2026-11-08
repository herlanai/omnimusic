package main

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

type audiusTrack struct {
	ID, Title, Permalink string
	Duration             float64
	User                 struct{ Name string }
	Available            bool `json:"is_available"`
	Streamable           bool `json:"is_streamable"`
	Gated                bool `json:"is_stream_gated"`
}

func fetchAudius(ctx context.Context, channel, language string, page int) ([]track, error) {
	params := url.Values{"app_name": {"OmniMusic"}, "limit": {"100"}, "offset": {strconv.Itoa((page - 1) * 100)}}
	endpoint := "https://api.audius.co/v1/tracks/trending"
	switch channel {
	case "钢琴":
		endpoint = "https://api.audius.co/v1/tracks/search"
		params.Set("query", "piano")
	case "纯音乐":
		params.Set("genre", "Ambient")
	default:
		params.Set("genre", "Pop")
	}
	if channel == "每日精选" && page > 1 {
		endpoint = "https://api.audius.co/v1/tracks/search"
		params.Set("query", "pop")
		params.Del("genre")
	}
	urls := []string{endpoint + "?" + params.Encode()}
	if language == "zh" && channel == "每日精选" {
		p := url.Values{"app_name": {"OmniMusic"}, "query": {"中文"}, "limit": {"100"}, "offset": {strconv.Itoa((page - 1) * 100)}}
		urls = append(urls, "https://api.audius.co/v1/tracks/search?"+p.Encode())
	}
	type result struct {
		rows []audiusTrack
		err  error
	}
	results := make(chan result, len(urls))
	for _, address := range urls {
		go func() {
			var response struct{ Data []audiusTrack }
			err := remoteJSON(ctx, address, &response)
			results <- result{response.Data, err}
		}()
	}
	var out []track
	var failures []error
	seen := map[string]bool{}
	for range urls {
		r := <-results
		if r.err != nil {
			failures = append(failures, r.err)
			continue
		}
		for _, song := range r.rows {
			if song.ID == "" || seen[song.ID] || !song.Streamable || !song.Available || song.Gated || song.Duration <= 0 {
				continue
			}
			seen[song.ID] = true
			group := "独立流行"
			if channel != "每日精选" {
				group = channel
			}
			lang := ""
			// Audius does not expose a vocal-language field. Chinese search results
			// with Chinese titles receive a discovery hint, not an audio-language claim.
			if strings.ContainsFunc(song.Title, func(r rune) bool { return unicode.Is(unicode.Han, r) }) {
				lang = "zh"
			}
			out = append(out, track{Title: song.Title, Artist: song.User.Name, Seconds: song.Duration, Format: "MP3",
				Remote: true, StreamOnly: true, Language: lang, Group: group,
				Source:  "https://api.audius.co/v1/tracks/" + url.PathEscape(song.ID) + "/stream?app_name=OmniMusic",
				Release: "https://audius.co" + song.Permalink})
		}
	}
	if len(out) == 0 {
		return nil, errors.Join(append(failures, errors.New("Audius returned no public playable tracks"))...)
	}
	return out, nil
}
