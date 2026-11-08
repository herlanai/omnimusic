package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/HyperparameterSI/golet/backend/plugins/sqlite"
)

const recommendationCooldown = 7 * 24 * time.Hour

// Versions of the same named recording share a cooldown across providers.
func recordingID(t track) string {
	normalize := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsSpace(r) || unicode.IsPunct(r) {
				return -1
			}
			return unicode.ToLower(r)
		}, s)
	}
	key := normalize(t.Title) + "|" + normalize(t.Artist+" "+t.Composer)
	if key == "|" {
		key = t.Source
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}

type cachedPick struct {
	ID    string
	Track track
}

func recommendationDB(path string) (*sqlite.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sqlite.Open(context.Background(), path, sqlite.OpenOptions{BusyTimeout: time.Second})
	if err != nil {
		return nil, err
	}
	_, err = db.Transaction(context.Background(), []sqlite.Statement{
		{SQL: `CREATE TABLE IF NOT EXISTS tracks (
			id TEXT NOT NULL, provider_id TEXT NOT NULL, provider TEXT NOT NULL,
			title TEXT NOT NULL, artist TEXT NOT NULL, url TEXT NOT NULL,
			language TEXT NOT NULL, metadata TEXT NOT NULL, rank REAL NOT NULL,
			PRIMARY KEY(id, url))`},
		{SQL: `CREATE TABLE IF NOT EXISTS recommendation_history (
			id TEXT PRIMARY KEY, first_recommended INTEGER NOT NULL,
			last_recommended INTEGER NOT NULL)`},
		{SQL: `CREATE TABLE IF NOT EXISTS recommendation_batches (
			sequence INTEGER PRIMARY KEY AUTOINCREMENT, token TEXT UNIQUE NOT NULL,
			day TEXT NOT NULL, channel TEXT NOT NULL, language TEXT NOT NULL,
			picks TEXT NOT NULL)`},
		{SQL: "CREATE INDEX IF NOT EXISTS provider_identity ON tracks(provider,provider_id)"},
		{SQL: `CREATE INDEX IF NOT EXISTS batches_lookup
			ON recommendation_batches(day, channel, language, sequence)`},
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Selection and reservation run in one SQLite write transaction so simultaneous
// windows cannot draw the same eligible recording into different fresh batches.
func cachedRecommendations(path, day, channel, language string, now time.Time, refresh bool, pool []track, evidence map[string]topicEvidence) ([]track, error) {
	db, err := recommendationDB(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	token := rand.Text()
	readBatch := func() ([]track, bool, error) {
		rows, err := db.Query(ctx, `SELECT picks FROM recommendation_batches
			WHERE day=? AND channel=? AND language=? AND (?=0 OR token=?) ORDER BY sequence DESC LIMIT 1`, day, channel, language, refresh, token)
		if err != nil || len(rows.Values) == 0 {
			return nil, false, err
		}
		var picks []cachedPick
		if err := json.Unmarshal([]byte(rows.Values[0][0].(string)), &picks); err != nil {
			return nil, false, err
		}
		out := make([]track, 0, len(picks))
		for _, p := range picks {
			out = append(out, p.Track)
		}
		return out, true, nil
	}
	if !refresh {
		if out, found, err := readBatch(); found || err != nil {
			return out, err
		}
	}
	statements := []sqlite.Statement{{SQL: "CREATE TEMP TABLE eligible (id TEXT, url TEXT, PRIMARY KEY(id,url))"}}
	for _, t := range pool {
		if t.Format == youtube {
			t.Source = youtubeURL(t.Source)
		}
		if t.Source == "" {
			continue
		}
		u, err := url.Parse(t.Source)
		if err != nil || u.Hostname() == "" || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		provider := u.Hostname()
		providerID := t.Digest
		if t.Format == youtube {
			provider = "youtube"
			providerID = u.Query().Get("v")
		}
		if providerID == "" {
			providerID = t.Source
		}
		t.Language, _, _ = strings.Cut(strings.ToLower(t.Language), "-")
		id := recordingID(t)
		metadata, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		rank := sampleRank(day+"/"+token, id, 1+math.Log1p(evidence[t.Group].strength))
		statements = append(statements,
			sqlite.Statement{SQL: `INSERT INTO tracks VALUES(?,?,?,?,?,?,?,?,?)
				ON CONFLICT(id,url) DO UPDATE SET provider_id=excluded.provider_id,
				provider=excluded.provider,title=excluded.title,artist=excluded.artist,
				language=excluded.language,metadata=excluded.metadata,rank=excluded.rank`,
				Args: []any{id, providerID, provider, t.Title, t.Artist, t.Source, t.Language, string(metadata), rank}},
			sqlite.Statement{SQL: "INSERT OR IGNORE INTO eligible VALUES(?,?)", Args: []any{id, t.Source}},
		)
	}
	statements = append(statements,
		sqlite.Statement{SQL: `INSERT INTO recommendation_batches(token,day,channel,language,picks)
			SELECT ?,?,?,?,(
				SELECT json_group_array(json_object('ID',id,'Track',json(metadata))) FROM (
					SELECT t.id,t.metadata,t.language,MIN(t.rank) AS priority FROM tracks t
					JOIN eligible e ON t.id=e.id AND t.url=e.url
					WHERE NOT EXISTS (
						SELECT 1 FROM recommendation_history h JOIN tracks old ON old.id=h.id
						WHERE h.last_recommended>? AND (h.id=t.id OR
							(old.provider=t.provider AND old.provider_id=t.provider_id)))
					GROUP BY t.id
					ORDER BY (t.language=?) DESC,priority,t.id LIMIT 12))
			WHERE ? OR NOT EXISTS(SELECT 1 FROM recommendation_batches WHERE day=? AND channel=? AND language=?)`,
			Args: []any{token, day, channel, language, now.Add(-recommendationCooldown).Unix(), language, refresh, day, channel, language}},
		sqlite.Statement{SQL: `INSERT INTO recommendation_history(id,first_recommended,last_recommended)
			SELECT json_extract(j.value,'$.ID'),?,? FROM recommendation_batches b,json_each(b.picks) j
			WHERE b.token=?
			ON CONFLICT(id) DO UPDATE SET last_recommended=excluded.last_recommended`,
			Args: []any{now.Unix(), now.Unix(), token}},
	)
	statements = append(statements, sqlite.Statement{SQL: "DELETE FROM recommendation_batches WHERE token=? AND picks='[]'", Args: []any{token}})
	if _, err := db.Transaction(ctx, statements); err != nil {
		return nil, err
	}
	out, _, err := readBatch()
	return out, err
}
