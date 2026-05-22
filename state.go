package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/jcalabro/atmos"
	"github.com/jcalabro/atmos/cbor"
	"github.com/jcalabro/atmos/sync"

	_ "github.com/mattn/go-sqlite3"
)

// sqliteStateStore is a small, durable [sync.StateStore] backed by
// SQLite. It persists per-DID chain and hosting state across restarts
// so that the verifier doesn't accept the next event for each DID as
// ground truth after a process restart (which is what MemStateStore
// effectively does).
//
// Layout: a single table keyed by DID with chain and hosting columns
// kept side-by-side. The interface is field-targeted (LoadChain /
// SaveChain / LoadHosting / SaveHosting / Delete), so we use UPSERTs
// that touch only the relevant columns and avoid a read-modify-write.
type sqliteStateStore struct {
	db *sql.DB
}

// schema for the single state table. The CID is stored as a TEXT
// (multibase string) for human-debuggability; the data volume here is
// tiny (one row per DID we've ever seen) so we don't need the binary
// form.
const stateStoreSchema = `
CREATE TABLE IF NOT EXISTS state (
	did             TEXT PRIMARY KEY,
	chain_rev       TEXT,
	chain_data      TEXT,
	hosting_active  INTEGER,
	hosting_status  TEXT,
	hosting_seq     INTEGER,
	hosting_time    TEXT,
	hosting_present INTEGER NOT NULL DEFAULT 0,
	chain_present   INTEGER NOT NULL DEFAULT 0
) WITHOUT ROWID;

-- Single-row table holding the firehose cursor. Keyed by a constant
-- "id = 0" CHECK so we can UPSERT without ever growing past one row.
CREATE TABLE IF NOT EXISTS cursor (
	id  INTEGER PRIMARY KEY CHECK (id = 0),
	seq INTEGER NOT NULL
);`

func openSQLiteStateStoreIfFirehose(rawURL string, path string) (*sqliteStateStore, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}

	isFirehose := strings.EqualFold(u.Path, "/xrpc/com.atproto.sync.subscribeRepos")
	if !isFirehose {
		return nil, nil
	}

	return openSQLiteStateStore(path)
}

// openSQLiteStateStore opens or creates a SQLite database at path and
// applies the schema. The caller is responsible for Close().
func openSQLiteStateStore(path string) (*sqliteStateStore, error) {
	// _journal_mode=WAL gives much better behavior under concurrent
	// reads (the verifier may be touching state from multiple
	// goroutines). _busy_timeout avoids spurious "database is locked"
	// errors if a write briefly contends with itself.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000", path)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if _, err := db.ExecContext(context.Background(), stateStoreSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("apply state schema: %w", err)
	}
	return &sqliteStateStore{db: db}, nil
}

func (s *sqliteStateStore) Close() error {
	return s.db.Close()
}

// LoadChain returns the chain state for did, or (nil, nil) if absent.
func (s *sqliteStateStore) LoadChain(ctx context.Context, did atmos.DID) (*sync.ChainState, error) {
	var (
		rev     sql.NullString
		data    sql.NullString
		present int
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT chain_rev, chain_data, chain_present FROM state WHERE did = ?`,
		string(did),
	).Scan(&rev, &data, &present)
	if err == sql.ErrNoRows || present == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load chain %s: %w", did, err)
	}

	cid, err := cbor.ParseCIDString(data.String)
	if err != nil {
		return nil, fmt.Errorf("load chain %s: parse data CID: %w", did, err)
	}
	return &sync.ChainState{Rev: rev.String, Data: cid}, nil
}

// SaveChain records the chain state for did, leaving any existing
// hosting state untouched.
func (s *sqliteStateStore) SaveChain(ctx context.Context, did atmos.DID, state sync.ChainState) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO state (did, chain_rev, chain_data, chain_present)
		 VALUES (?, ?, ?, 1)
		 ON CONFLICT(did) DO UPDATE SET
		   chain_rev = excluded.chain_rev,
		   chain_data = excluded.chain_data,
		   chain_present = 1`,
		string(did), state.Rev, state.Data.String(),
	)
	if err != nil {
		return fmt.Errorf("save chain %s: %w", did, err)
	}
	return nil
}

// LoadHosting returns the hosting state for did, or (nil, nil) if absent.
func (s *sqliteStateStore) LoadHosting(ctx context.Context, did atmos.DID) (*sync.HostingState, error) {
	var (
		active  sql.NullInt64
		status  sql.NullString
		seq     sql.NullInt64
		ts      sql.NullString
		present int
	)
	err := s.db.QueryRowContext(ctx,
		`SELECT hosting_active, hosting_status, hosting_seq, hosting_time, hosting_present
		   FROM state WHERE did = ?`,
		string(did),
	).Scan(&active, &status, &seq, &ts, &present)
	if err == sql.ErrNoRows || present == 0 {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load hosting %s: %w", did, err)
	}

	return &sync.HostingState{
		Active: active.Int64 != 0,
		Status: status.String,
		Seq:    seq.Int64,
		Time:   ts.String,
	}, nil
}

// SaveHosting records the hosting state for did, leaving any existing
// chain state untouched.
func (s *sqliteStateStore) SaveHosting(ctx context.Context, did atmos.DID, state sync.HostingState) error {
	active := 0
	if state.Active {
		active = 1
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO state (did, hosting_active, hosting_status, hosting_seq, hosting_time, hosting_present)
		 VALUES (?, ?, ?, ?, ?, 1)
		 ON CONFLICT(did) DO UPDATE SET
		   hosting_active = excluded.hosting_active,
		   hosting_status = excluded.hosting_status,
		   hosting_seq = excluded.hosting_seq,
		   hosting_time = excluded.hosting_time,
		   hosting_present = 1`,
		string(did), active, state.Status, state.Seq, state.Time,
	)
	if err != nil {
		return fmt.Errorf("save hosting %s: %w", did, err)
	}
	return nil
}

// Delete removes both chain and hosting state for did atomically.
func (s *sqliteStateStore) Delete(ctx context.Context, did atmos.DID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM state WHERE did = ?`, string(did))
	if err != nil {
		return fmt.Errorf("delete state %s: %w", did, err)
	}
	return nil
}

// LoadCursor returns the persisted firehose cursor, or 0 if none has
// been stored yet. Implements [streaming.CursorStore].
func (s *sqliteStateStore) LoadCursor(ctx context.Context) (int64, error) {
	var seq int64
	err := s.db.QueryRowContext(ctx, `SELECT seq FROM cursor WHERE id = 0`).Scan(&seq)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load cursor: %w", err)
	}
	return seq, nil
}

// SaveCursor durably writes the firehose cursor. The streaming client
// calls this every CursorInterval events and once on Close, so it is
// not on the per-event hot path. Implements [streaming.CursorStore].
func (s *sqliteStateStore) SaveCursor(ctx context.Context, cursor int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO cursor (id, seq) VALUES (0, ?)
		 ON CONFLICT(id) DO UPDATE SET seq = excluded.seq`,
		cursor,
	)
	if err != nil {
		return fmt.Errorf("save cursor: %w", err)
	}
	return nil
}
