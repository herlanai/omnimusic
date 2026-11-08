package main

import (
	"context"
	encodinghex "encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

var catalogHTTP = &http.Client{Timeout: 12 * time.Second, Transport: &http.Transport{
	Proxy: http.ProxyFromEnvironment, MaxConnsPerHost: 6,
}}

func remoteJSON(ctx context.Context, address string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "OmniMusic/0.1.49")
	resp, err := catalogHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("music service HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}

// Archive metadata accepts both scalar and array values.
type archiveText string

func (s *archiveText) UnmarshalJSON(b []byte) error {
	var value any
	if err := json.Unmarshal(b, &value); err != nil {
		return err
	}
	switch v := value.(type) {
	case string:
		*s = archiveText(v)
	case float64:
		*s = archiveText(strconv.FormatFloat(v, 'f', -1, 64))
	case []any:
		var parts []string
		for _, p := range v {
			if str, ok := p.(string); ok {
				parts = append(parts, str)
			}
		}
		*s = archiveText(strings.Join(parts, "; "))
	}
	return nil
}

type archiveItem struct {
	Metadata struct{ Title, Creator, Language, Licenseurl archiveText }
	Files    []struct {
		Name, Title, Creator, Artist, Format, SHA1 string
		Size, Length                               archiveText
	}
}

func archiveTracks(item archiveItem, identifier, channel string) []track {
	license := string(item.Metadata.Licenseurl)
	if !strings.Contains(license, "creativecommons.org/licenses/") && !strings.Contains(license, "creativecommons.org/publicdomain/") {
		return nil
	}
	lang := ""
	switch strings.ToLower(string(item.Metadata.Language)) {
	case "chinese", "mandarin", "zh", "zho", "chi", "zh-cn", "zh-hans":
		lang = "zh"
	case "english", "eng", "en":
		lang = "en"
	}
	group := "独立流行"
	if channel == "钢琴" {
		group = "钢琴"
	}
	if channel == "纯音乐" {
		group = "纯音乐"
	}
	var out []track
	for _, f := range item.Files {
		format := ""
		switch strings.ToLower(path.Ext(f.Name)) {
		case ".mp3":
			format = "MP3"
		case ".flac":
			format = "FLAC"
		}
		hash, err := encodinghex.DecodeString(f.SHA1)
		size, _ := strconv.ParseInt(string(f.Size), 10, 64)
		if format == "" || err != nil || len(hash) != 20 || size <= 0 {
			continue
		}
		title := f.Title
		if title == "" {
			title = strings.TrimSuffix(path.Base(f.Name), path.Ext(f.Name))
		}
		artist := f.Artist
		if artist == "" {
			artist = f.Creator
		}
		if artist == "" {
			artist = string(item.Metadata.Creator)
		}
		duration := 0.
		for _, part := range strings.Split(string(f.Length), ":") {
			n, _ := strconv.ParseFloat(part, 64)
			duration = duration*60 + n
		}
		out = append(out, track{Title: title, Artist: artist, Album: string(item.Metadata.Title), Language: lang,
			Format: format, Digest: strings.ToLower(f.SHA1), Size: size, Seconds: duration, Group: group,
			Source:  "https://archive.org/download/" + url.PathEscape(identifier) + "/" + url.PathEscape(f.Name),
			Release: "https://archive.org/details/" + url.PathEscape(identifier), License: license, Remote: true})
	}
	return out
}

func fetchArchive(ctx context.Context, channel, language string, page int) ([]track, error) {
	tag := "(subject:pop OR subject:indie OR subject:rock OR subject:song)"
	if channel == "钢琴" {
		tag = "subject:piano"
	}
	if channel == "纯音乐" {
		tag = "(subject:instrumental OR subject:ambient)"
	}
	base := "mediatype:audio AND collection:netlabels AND licenseurl:* AND " + tag
	preferred := "(language:eng OR language:English OR language:en)"
	if language == "zh" {
		preferred = "(language:chi OR language:Chinese OR language:zh OR language:zho OR language:Mandarin)"
	}

	seen := map[string]bool{}
	var identifiers []string
	var failures []error
	for _, query := range []string{base + " AND " + preferred, base} {
		params := url.Values{"q": {query}, "output": {"json"}, "rows": {"24"}, "page": {strconv.Itoa(page)}, "fl[]": {"identifier"}, "sort[]": {"publicdate desc"}}
		var search struct {
			Response struct {
				NumFound int
				Docs     []struct{ Identifier string }
			}
		}
		if err := remoteJSON(ctx, "https://archive.org/advancedsearch.php?"+params.Encode(), &search); err != nil {
			failures = append(failures, err)
			continue
		}
		if len(search.Response.Docs) == 0 && search.Response.NumFound > 0 {
			params.Set("page", strconv.Itoa(1+(page-1)%((search.Response.NumFound+23)/24)))
			_ = remoteJSON(ctx, "https://archive.org/advancedsearch.php?"+params.Encode(), &search)
		}
		for _, doc := range search.Response.Docs {
			if !seen[doc.Identifier] {
				seen[doc.Identifier] = true
				identifiers = append(identifiers, doc.Identifier)
			}
		}
	}
	type result struct {
		tracks []track
		err    error
	}
	results := make(chan result, len(identifiers))
	jobs := make(chan string, len(identifiers))
	for _, id := range identifiers {
		jobs <- id
	}
	close(jobs)
	for range min(4, len(identifiers)) {
		go func() {
			for id := range jobs {
				var item archiveItem
				err := remoteJSON(ctx, "https://archive.org/metadata/"+url.PathEscape(id), &item)
				tracks := archiveTracks(item, id, channel)
				results <- result{tracks[:min(8, len(tracks))], err}
			}
		}()
	}
	var out []track
	for range identifiers {
		r := <-results
		if r.err != nil {
			failures = append(failures, r.err)
		}
		out = append(out, r.tracks...)
	}
	if len(out) == 0 {
		return nil, errors.Join(append(failures, errors.New("no playable licensed tracks returned"))...)
	}
	return out, nil
}

func fetchYouTube(ctx context.Context, language string, page int) ([]track, error) {
	query := "music"
	if language == "zh" {
		query = "music Chinese"
	}
	params := url.Values{"query": {query}, "tags": {"story"}, "restrictSearchableAttributes": {"title"}, "hitsPerPage": {"200"}, "page": {strconv.Itoa(page - 1)}}
	var search struct{ Hits []struct{ URL string } }
	if err := remoteJSON(ctx, "https://hn.algolia.com/api/v1/search?"+params.Encode(), &search); err != nil {
		return nil, err
	}
	if language == "zh" {
		params.Set("query", "music")
		var fallback struct{ Hits []struct{ URL string } }
		if remoteJSON(ctx, "https://hn.algolia.com/api/v1/search?"+params.Encode(), &fallback) == nil {
			search.Hits = append(search.Hits, fallback.Hits...)
		}
	}
	var out []track
	seen := map[string]bool{}
	for _, hit := range search.Hits {
		source := youtubeURL(hit.URL)
		if source == "" || seen[source] {
			continue
		}
		seen[source] = true
		var meta struct {
			Title  string
			Author string `json:"author_name"`
		}
		if err := remoteJSON(ctx, "https://www.youtube.com/oembed?format=json&url="+url.QueryEscape(source), &meta); err != nil {
			continue
		}
		if meta.Title == "" {
			continue
		}
		out = append(out, track{Title: meta.Title, Artist: meta.Author, Source: source, Format: youtube, Group: "独立流行", Remote: true})
		if len(out) == 24 {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no playable YouTube music links returned by HN")
	}
	return out, nil
}

type remoteResult struct {
	channel, date, language string
	refresh                 bool
	tracks                  []track
	err                     error
}

func (s *studio) requestRemote(channel string, refresh bool) {
	if !refresh {
		s.restoreRemoteBatch(channel)
	}
	needs := refresh || len(s.remoteRows(channel)) == 0
	if needs {
		s.remotePending[channel] = !s.selectRemote(channel, refresh)
	}
	if s.remoteLoading[channel] {
		return
	}
	// Load other categories when opened, keeping startup bandwidth for the visible page.
	if channel != s.page && !refresh {
		return
	}
	s.remoteLoading[channel] = true
	delete(s.recommendationErrors, channel)
	date, lang := s.date, s.mixLanguage
	page, err := nextRemotePage(s.recommendationPath, channel+"/"+lang)
	if err != nil {
		s.remoteLoading[channel] = false
		s.recommendationErrors[channel] = err.Error()
		return
	}
	providers := []func(context.Context, string, string, int) ([]track, error){fetchAudius, fetchArchive}
	if channel == youtube {
		providers = []func(context.Context, string, string, int) ([]track, error){
			func(ctx context.Context, _ string, lang string, page int) ([]track, error) {
				return fetchYouTube(ctx, lang, page)
			},
		}
	}
	s.remoteWorkers[channel] = len(providers)
	for _, provider := range providers {
		go func() {
			ctx, cancel := context.WithTimeout(s.remoteContext, 20*time.Second)
			defer cancel()
			tracks, err := provider(ctx, channel, lang, page)
			select {
			case s.remoteResults <- remoteResult{channel, date, lang, refresh, tracks, err}:
			case <-s.remoteContext.Done():
			}
		}()
	}
}

func (s *studio) pollRemote() {
	if s.remoteResults == nil {
		return
	}
	select {
	case result := <-s.remoteResults:
		s.remoteWorkers[result.channel]--
		s.remoteLoading[result.channel] = s.remoteWorkers[result.channel] > 0
		if result.date != s.date || result.language != s.mixLanguage {
			if !s.remoteLoading[result.channel] {
				s.requestRemote(result.channel, false)
			}
			return
		}
		if result.err != nil {
			if len(s.remotePool(result.channel)) == 0 {
				s.recommendationErrors[result.channel] = result.err.Error()
			}
			return
		}
		if err := s.storeRemoteInventory(result.channel, result.tracks); err != nil {
			s.recommendationErrors[result.channel] = err.Error()
		}
		s.catalog = mergeRemoteTracks(s.catalog, result.tracks)
		s.remotePools[result.channel] = mergeRemoteTracks(s.remotePools[result.channel], result.tracks)
		if s.remotePending[result.channel] || len(s.remoteRows(result.channel)) == 0 {
			s.selectRemote(result.channel, true)
		}
	default:
	}
}

func mergeRemoteTracks(existing, incoming []track) []track {
	index := map[string]int{}
	for i, t := range existing {
		index[t.Source] = i
	}
	for _, t := range incoming {
		if i, ok := index[t.Source]; ok {
			existing[i] = t
		} else {
			index[t.Source] = len(existing)
			existing = append(existing, t)
		}
	}
	return existing
}

// Restore only metadata previously fetched from online providers.
func (s *studio) restoreRemoteCatalog() {
	db, err := recommendationDB(s.recommendationPath)
	if err != nil {
		return
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "SELECT metadata FROM tracks WHERE json_extract(metadata,'$.Remote')=1")
	if err != nil {
		return
	}
	for _, r := range rows.Values {
		var t track
		if json.Unmarshal([]byte(r[0].(string)), &t) == nil {
			s.catalog = append(s.catalog, t)
		}
	}
}

func (s *studio) restoreRemoteBatch(channel string) {
	db, err := recommendationDB(s.recommendationPath)
	if err != nil {
		return
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), `SELECT picks FROM recommendation_batches
		WHERE channel=? AND day=? AND language=? ORDER BY sequence DESC LIMIT 1`, "remote-v1/"+channel, s.date, s.mixLanguage)
	if err != nil || len(rows.Values) == 0 {
		return
	}
	var picks []cachedPick
	if json.Unmarshal([]byte(rows.Values[0][0].(string)), &picks) != nil {
		return
	}
	var tracks []track
	for _, p := range picks {
		if p.Track.Remote {
			tracks = append(tracks, p.Track)
		}
	}
	switch channel {
	case youtube:
		s.videoMix = tracks
	case "每日精选":
		s.mix = tracks
	default:
		if s.genreMix == nil {
			s.genreMix = map[string][]track{}
		}
		s.genreMix[channel] = tracks
	}
}

func nextRemotePage(dbPath, key string) (int, error) {
	db, err := recommendationDB(dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	ctx := context.Background()
	if _, err = db.Execute(ctx, "CREATE TABLE IF NOT EXISTS remote_pages (source TEXT PRIMARY KEY, page INTEGER NOT NULL)"); err != nil {
		return 0, err
	}
	rows, err := db.Query(ctx, `INSERT INTO remote_pages VALUES(?,1)
		ON CONFLICT(source) DO UPDATE SET page=page+1 RETURNING page`, key)
	if err != nil {
		return 0, err
	}
	return int(rows.Values[0][0].(int64)), nil
}
