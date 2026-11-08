package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/HyperparameterSI/golet/backend/plugins/sqlite"
)

func (s *studio) remotePool(channel string) []track { return s.remotePools[channel] }

func (s *studio) storeRemoteInventory(channel string, tracks []track) error {
	db, err := recommendationDB(s.recommendationPath)
	if err != nil {
		return err
	}
	defer db.Close()
	statements := []sqlite.Statement{{SQL: `CREATE TABLE IF NOT EXISTS remote_inventory
		(channel TEXT,source TEXT,metadata TEXT,fetched_at INTEGER,PRIMARY KEY(channel,source))`}}
	for _, song := range tracks {
		data, err := json.Marshal(song)
		if err != nil {
			return err
		}
		statements = append(statements, sqlite.Statement{SQL: `INSERT INTO remote_inventory VALUES(?,?,?,?)
		ON CONFLICT(channel,source) DO UPDATE SET metadata=excluded.metadata,fetched_at=excluded.fetched_at`,
			Args: []any{channel, song.Source, string(data), time.Now().Unix()}})
	}
	statements = append(statements, sqlite.Statement{SQL: "DELETE FROM remote_inventory WHERE fetched_at<?", Args: []any{time.Now().Add(-14 * 24 * time.Hour).Unix()}})
	_, err = db.Transaction(context.Background(), statements)
	return err
}

func (s *studio) restoreRemoteInventory() {
	if err := s.storeRemoteInventory("", nil); err != nil {
		return
	}
	db, err := recommendationDB(s.recommendationPath)
	if err != nil {
		return
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "SELECT channel,metadata FROM remote_inventory ORDER BY fetched_at DESC LIMIT 3000")
	if err != nil {
		return
	}
	for _, r := range rows.Values {
		var t track
		if json.Unmarshal([]byte(r[1].(string)), &t) == nil && t.Remote {
			channel := r[0].(string)
			s.remotePools[channel] = append(s.remotePools[channel], t)
			s.catalog = mergeRemoteTracks(s.catalog, []track{t})
		}
	}
}

func (s *studio) remoteRows(channel string) []track {
	switch channel {
	case youtube:
		return s.videoMix
	case "每日精选":
		return s.mix
	default:
		return s.genreMix[channel]
	}
}

func (s *studio) setRemoteRows(channel string, rows []track) {
	switch channel {
	case youtube:
		s.videoMix = rows
	case "每日精选":
		s.mix = rows
		if s.current.Source == "" && len(rows) > 0 {
			s.current = rows[0]
			s.queue = append([]track(nil), rows...)
		}
	default:
		s.genreMix[channel] = rows
	}
}

// A failed/empty refresh keeps the visible batch. New arrivals complete the
// pending refresh once, rather than replacing a list the user already switched.
func (s *studio) selectRemote(channel string, refresh bool) bool {
	pool := s.remotePool(channel)
	if len(pool) == 0 {
		return false
	}
	rows := s.cachedMix(channel, refresh, pool)
	if len(rows) == 0 {
		return false
	}
	s.setRemoteRows(channel, rows)
	s.remotePending[channel] = false
	return true
}
